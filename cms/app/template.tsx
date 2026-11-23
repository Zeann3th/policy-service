// Remounted on every navigation, so each page rises in.
export default function Template({ children }: { children: React.ReactNode }) {
  return <div className="enter">{children}</div>;
}
