import './Toast.css';

export function Toast({ message, level, visible }: { message: string; level: 'error' | 'info'; visible: boolean }) {
  return (
    <div class={`toast ${level}${visible ? ' show' : ''}`} role="status">
      {message}
    </div>
  );
}
