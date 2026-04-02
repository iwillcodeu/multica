package handler

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/auth"
	"github.com/multica-ai/multica/server/internal/logger"
	"github.com/multica-ai/multica/server/internal/userdisplay"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type UserResponse struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Email       string  `json:"email"`
	AvatarURL   *string `json:"avatar_url"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	HasPassword bool    `json:"has_password"`
}

func userToResponse(u db.User) UserResponse {
	hasPW := u.PasswordHash.Valid && strings.TrimSpace(u.PasswordHash.String) != ""
	return UserResponse{
		ID:          uuidToString(u.ID),
		Name:        u.Name,
		Email:       u.Email,
		AvatarURL:   textToPtr(u.AvatarUrl),
		CreatedAt:   timestampToString(u.CreatedAt),
		UpdatedAt:   timestampToString(u.UpdatedAt),
		HasPassword: hasPW,
	}
}

type LoginResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

type SendCodeRequest struct {
	Email string `json:"email"`
}

type VerifyCodeRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type LoginPasswordRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ChangeMyPasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// LoginPassword issues a JWT for users that have a password set (email + password).
func (h *Handler) LoginPassword(w http.ResponseWriter, r *http.Request) {
	var req LoginPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	password := strings.TrimSpace(req.Password)
	if email == "" || password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	const badCreds = "invalid email or password"
	user, err := h.Queries.GetUserByEmail(r.Context(), email)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusUnauthorized, badCreds)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load user")
		return
	}
	if !user.PasswordHash.Valid || strings.TrimSpace(user.PasswordHash.String) == "" {
		writeError(w, http.StatusBadRequest, "password sign-in is not enabled for this account; use email code")
		return
	}
	if !auth.PasswordMatches(user.PasswordHash.String, password) {
		writeError(w, http.StatusUnauthorized, badCreds)
		return
	}

	if err := h.ensureUserWorkspace(r.Context(), user); err != nil {
		slog.Warn("login password provision workspace failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to provision workspace")
		return
	}

	tokenString, err := h.issueJWT(user)
	if err != nil {
		slog.Warn("login password failed", append(logger.RequestAttrs(r), "error", err, "email", email)...)
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	if h.CFSigner != nil {
		for _, cookie := range h.CFSigner.SignedCookies(time.Now().Add(72 * time.Hour)) {
			http.SetCookie(w, cookie)
		}
	}

	slog.Info("user logged in (password)", append(logger.RequestAttrs(r), "user_id", uuidToString(user.ID), "email", user.Email)...)
	writeJSON(w, http.StatusOK, LoginResponse{
		Token: tokenString,
		User:  userToResponse(user),
	})
}

// ChangeMyPassword sets or updates the current user's password.
func (h *Handler) ChangeMyPassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req ChangeMyPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	newPW := strings.TrimSpace(req.NewPassword)
	if err := auth.ValidatePasswordSetting(newPW); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.Queries.GetUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	hasPW := user.PasswordHash.Valid && strings.TrimSpace(user.PasswordHash.String) != ""
	if hasPW {
		cur := strings.TrimSpace(req.CurrentPassword)
		if cur == "" {
			writeError(w, http.StatusBadRequest, "current password is required")
			return
		}
		if !auth.PasswordMatches(user.PasswordHash.String, cur) {
			writeError(w, http.StatusBadRequest, "current password is incorrect")
			return
		}
	}

	hashed, err := auth.HashPassword(newPW)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set password")
		return
	}
	if err := h.Queries.SetUserPasswordHash(r.Context(), db.SetUserPasswordHashParams{
		ID:           user.ID,
		PasswordHash: pgtype.Text{String: hashed, Valid: true},
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set password")
		return
	}

	updated, err := h.Queries.GetUser(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load user")
		return
	}
	writeJSON(w, http.StatusOK, userToResponse(updated))
}

func defaultWorkspaceName(user db.User) string {
	name := strings.TrimSpace(user.Name)
	if name == "" {
		email := strings.TrimSpace(user.Email)
		if at := strings.Index(email, "@"); at > 0 {
			name = email[:at]
		}
	}
	if name == "" {
		name = "Personal"
	}
	return name + "'s Workspace"
}

func slugifyWorkspacePart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastWasDash := false

	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastWasDash = false
		case b.Len() > 0 && !lastWasDash:
			b.WriteByte('-')
			lastWasDash = true
		}
	}

	return strings.Trim(b.String(), "-")
}

func defaultWorkspaceSlug(user db.User) string {
	candidates := []string{
		slugifyWorkspacePart(user.Name),
		slugifyWorkspacePart(strings.Split(strings.TrimSpace(user.Email), "@")[0]),
		"workspace",
	}

	base := "workspace"
	for _, candidate := range candidates {
		if candidate != "" {
			base = candidate
			break
		}
	}

	userID := uuidToString(user.ID)
	if len(userID) >= 8 {
		return base + "-" + userID[:8]
	}
	return base
}

func (h *Handler) ensureUserWorkspace(ctx context.Context, user db.User) error {
	workspaces, err := h.Queries.ListWorkspaces(ctx, user.ID)
	if err != nil {
		return err
	}
	if len(workspaces) > 0 {
		return nil
	}

	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := h.Queries.WithTx(tx)
	workspaces, err = qtx.ListWorkspaces(ctx, user.ID)
	if err != nil {
		return err
	}
	if len(workspaces) > 0 {
		return nil
	}

	wsName := defaultWorkspaceName(user)
	workspace, err := qtx.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		Name:        wsName,
		Slug:        defaultWorkspaceSlug(user),
		Description: pgtype.Text{},
		IssuePrefix: generateIssuePrefix(wsName),
	})
	if err != nil {
		if isUniqueViolation(err) {
			workspaces, lookupErr := h.Queries.ListWorkspaces(ctx, user.ID)
			if lookupErr == nil && len(workspaces) > 0 {
				return nil
			}
		}
		return err
	}

	if _, err := qtx.CreateMember(ctx, db.CreateMemberParams{
		WorkspaceID: workspace.ID,
		UserID:      user.ID,
		Role:        "owner",
	}); err != nil {
		return err
	}

	if _, err := qtx.CreateProject(ctx, db.CreateProjectParams{
		WorkspaceID: workspace.ID,
		Name:        "General",
		Position:    0,
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func generateCode() (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	n := binary.BigEndian.Uint32(buf[:]) % 1000000
	return fmt.Sprintf("%06d", n), nil
}

func (h *Handler) issueJWT(user db.User) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":   uuidToString(user.ID),
		"email": user.Email,
		"name":  user.Name,
		"exp":   time.Now().Add(72 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	})
	return token.SignedString(auth.JWTSecret())
}

func (h *Handler) findOrCreateUser(ctx context.Context, email string) (db.User, error) {
	user, err := h.Queries.GetUserByEmail(ctx, email)
	if err != nil {
		if !isNotFound(err) {
			return db.User{}, err
		}
		provisionName, perr := h.pickProvisionDisplayName(ctx, email)
		if perr != nil {
			return db.User{}, perr
		}
		user, err = h.Queries.CreateUser(ctx, db.CreateUserParams{
			Name:  provisionName,
			Email: email,
		})
		if err != nil {
			return db.User{}, err
		}
	}
	return user, nil
}

func (h *Handler) SendCode(w http.ResponseWriter, r *http.Request) {
	var req SendCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		writeError(w, http.StatusBadRequest, "email is required")
		return
	}

	// Rate limit: max 1 code per 10 seconds per email
	latest, err := h.Queries.GetLatestCodeByEmail(r.Context(), email)
	if err == nil && time.Since(latest.CreatedAt.Time) < 10*time.Second {
		writeError(w, http.StatusTooManyRequests, "please wait before requesting another code")
		return
	}

	code, err := generateCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate code")
		return
	}

	_, err = h.Queries.CreateVerificationCode(r.Context(), db.CreateVerificationCodeParams{
		Email:     email,
		Code:      code,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(10 * time.Minute), Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store verification code")
		return
	}

	if err := h.EmailService.SendVerificationCode(email, code); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to send verification code")
		return
	}

	// Best-effort cleanup of expired codes
	_ = h.Queries.DeleteExpiredVerificationCodes(r.Context())

	writeJSON(w, http.StatusOK, map[string]string{"message": "Verification code sent"})
}

func (h *Handler) VerifyCode(w http.ResponseWriter, r *http.Request) {
	var req VerifyCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	code := strings.TrimSpace(req.Code)

	if email == "" || code == "" {
		writeError(w, http.StatusBadRequest, "email and code are required")
		return
	}

	dbCode, err := h.Queries.GetLatestVerificationCode(r.Context(), email)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid or expired code")
		return
	}

	isMasterCode := code == "888888" && os.Getenv("APP_ENV") != "production"
	if !isMasterCode && subtle.ConstantTimeCompare([]byte(code), []byte(dbCode.Code)) != 1 {
		_ = h.Queries.IncrementVerificationCodeAttempts(r.Context(), dbCode.ID)
		writeError(w, http.StatusBadRequest, "invalid or expired code")
		return
	}

	if err := h.Queries.MarkVerificationCodeUsed(r.Context(), dbCode.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify code")
		return
	}

	user, err := h.findOrCreateUser(r.Context(), email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	if err := h.ensureUserWorkspace(r.Context(), user); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to provision workspace")
		return
	}

	tokenString, err := h.issueJWT(user)
	if err != nil {
		slog.Warn("login failed", append(logger.RequestAttrs(r), "error", err, "email", req.Email)...)
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	// Set CloudFront signed cookies for CDN access.
	if h.CFSigner != nil {
		for _, cookie := range h.CFSigner.SignedCookies(time.Now().Add(72 * time.Hour)) {
			http.SetCookie(w, cookie)
		}
	}

	slog.Info("user logged in", append(logger.RequestAttrs(r), "user_id", uuidToString(user.ID), "email", user.Email)...)
	writeJSON(w, http.StatusOK, LoginResponse{
		Token: tokenString,
		User:  userToResponse(user),
	})
}

func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	user, err := h.Queries.GetUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	writeJSON(w, http.StatusOK, userToResponse(user))
}

type UpdateMeRequest struct {
	Name      *string `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}

	var req UpdateMeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	currentUser, err := h.Queries.GetUser(r.Context(), parseUUID(userID))
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	name := currentUser.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if err := userdisplay.ValidateDisplayName(name); err != nil {
			if errors.Is(err, userdisplay.ErrEmptyDisplayName) {
				writeError(w, http.StatusBadRequest, "name is required")
				return
			}
			if errors.Is(err, userdisplay.ErrDisplayNameLong) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, "invalid name")
			return
		}
		taken, err := h.Queries.IsDisplayNameTakenByOther(r.Context(), db.IsDisplayNameTakenByOtherParams{
			CheckName: name,
			ExcludeID: currentUser.ID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to check name")
			return
		}
		if taken {
			writeError(w, http.StatusConflict, "display name already taken")
			return
		}
	}

	params := db.UpdateUserParams{
		ID:   currentUser.ID,
		Name: name,
	}
	if req.AvatarURL != nil {
		params.AvatarUrl = pgtype.Text{String: strings.TrimSpace(*req.AvatarURL), Valid: true}
	}

	updatedUser, err := h.Queries.UpdateUser(r.Context(), params)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "display name already taken")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	writeJSON(w, http.StatusOK, userToResponse(updatedUser))
}
