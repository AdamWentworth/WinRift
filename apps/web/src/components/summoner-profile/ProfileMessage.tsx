import type { ReactNode } from 'react';

export function ProfileMessage({ icon, title, body }: { icon?: ReactNode; title: string; body: string }) {
  return (
    <div className="profile-message">
      {icon}
      <div>
        <strong>{title}</strong>
        <span>{body}</span>
      </div>
    </div>
  );
}
