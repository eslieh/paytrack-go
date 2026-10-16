// Spinner is a 1em ring that inherits the button's text colour.
export function Spinner({ size = "1em" }) {
  return <span className="spinner" style={{ width: size, height: size }} aria-hidden="true" />;
}

// Busy is the in-progress label for a button: spinner + text, announced politely.
export function Busy({ children }) {
  return (
    <span className="busy" role="status" aria-live="polite">
      <Spinner />
      {children}
    </span>
  );
}
