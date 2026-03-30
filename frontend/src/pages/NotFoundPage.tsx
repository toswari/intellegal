import { Link } from "react-router-dom";

export function NotFoundPage() {
  return (
    <section className="page">
      <h2>Page not found</h2>
      <p>The requested page does not exist.</p>
      <Link to="/">Go to dashboard</Link>
    </section>
  );
}
