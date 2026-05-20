"use client";

import { MyIssuesProjectTabsRail } from "@multica/views/my-issues";

export default function MyIssuesLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-1 min-h-0">
      <MyIssuesProjectTabsRail />
      <div className="flex min-h-0 min-w-0 flex-1 flex-col">{children}</div>
    </div>
  );
}
