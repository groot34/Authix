function App() {
  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '2rem 1.25rem',
      }}
    >
      <main
        style={{
          maxWidth: 560,
          width: '100%',
          textAlign: 'center',
        }}
      >
        <div
          aria-hidden="true"
          style={{
            width: 64,
            height: 64,
            borderRadius: 16,
            margin: '0 auto 1.5rem',
            background:
              'linear-gradient(135deg, #6366f1 0%, #8b5cf6 50%, #ec4899 100%)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: 'white',
            fontSize: 28,
            fontWeight: 700,
            letterSpacing: 0.5,
            boxShadow: '0 10px 40px rgba(139, 92, 246, 0.35)',
          }}
        >
          A
        </div>

        <h1
          style={{
            fontSize: 'clamp(2rem, 5vw, 2.75rem)',
            margin: '0 0 0.75rem',
            fontWeight: 700,
            letterSpacing: -0.02,
          }}
        >
          Authix
        </h1>

        <p
          style={{
            margin: '0 auto 2rem',
            color: '#9aa3b2',
            fontSize: '1rem',
            maxWidth: 460,
          }}
        >
          A lean full-stack demo with email registration, one-time passcode
          login recognition, and a checkout flow. Built with React, Go, and
          PostgreSQL.
        </p>

        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))',
            gap: '0.75rem',
            marginBottom: '2.5rem',
          }}
        >
          <StackBadge label="React" />
          <StackBadge label="TypeScript" />
          <StackBadge label="Go" />
          <StackBadge label="PostgreSQL" />
        </div>

        <section
          aria-label="Project status"
          style={{
            padding: '1.25rem 1.5rem',
            borderRadius: 12,
            background: 'rgba(255,255,255,0.03)',
            border: '1px solid rgba(255,255,255,0.06)',
            textAlign: 'left',
          }}
        >
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              marginBottom: '0.5rem',
            }}
          >
            <h2
              style={{
                fontSize: '0.95rem',
                margin: 0,
                fontWeight: 600,
                letterSpacing: 0.01,
              }}
            >
              Development status
            </h2>
            <span
              style={{
                fontSize: '0.75rem',
                padding: '0.15rem 0.6rem',
                borderRadius: 999,
                background: 'rgba(99, 102, 241, 0.18)',
                color: '#c7d2fe',
                border: '1px solid rgba(99, 102, 241, 0.35)',
              }}
            >
              Phase 1
            </span>
          </div>
          <StatusList
            items={[
              { label: 'Foundation & project layout', done: true },
              { label: 'React + Vite frontend scaffold', done: true },
              { label: 'Go HTTP API with /health', done: true },
              { label: 'Local PostgreSQL via Docker Compose', done: true },
              { label: 'Registration + OTP flows', done: false },
              { label: 'Checkout form & user recognition', done: false },
              { label: 'Database schema & persistence', done: false },
              { label: 'Public deployment', done: false },
            ]}
          />
        </section>

        <footer
          style={{
            marginTop: '2rem',
            color: '#6b7280',
            fontSize: '0.8rem',
          }}
        >
          Full-stack engineering assessment · Work in progress
        </footer>
      </main>
    </div>
  );
}

function StackBadge({ label }: { label: string }) {
  return (
    <div
      style={{
        padding: '0.55rem 0.75rem',
        borderRadius: 10,
        border: '1px solid rgba(255,255,255,0.08)',
        background: 'rgba(255,255,255,0.03)',
        fontSize: '0.85rem',
        fontWeight: 500,
      }}
    >
      {label}
    </div>
  );
}

interface StatusItem {
  label: string;
  done: boolean;
}

function StatusList({ items }: { items: StatusItem[] }) {
  return (
    <ul
      style={{
        listStyle: 'none',
        padding: 0,
        margin: 0,
        display: 'flex',
        flexDirection: 'column',
        gap: '0.4rem',
      }}
    >
      {items.map((it) => (
        <li
          key={it.label}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '0.6rem',
            fontSize: '0.875rem',
            color: it.done ? '#d1d5db' : '#6b7280',
          }}
        >
          <span
            aria-hidden="true"
            style={{
              width: 16,
              height: 16,
              borderRadius: 999,
              border: `1.5px solid ${
                it.done ? 'rgb(52, 211, 153)' : 'rgba(255,255,255,0.18)'
              }`,
              background: it.done ? 'rgba(52, 211, 153, 0.15)' : 'transparent',
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: 11,
              color: 'rgb(52, 211, 153)',
              flexShrink: 0,
            }}
          >
            {it.done ? '✓' : ''}
          </span>
          <span
            style={{
              textDecoration: it.done ? undefined : undefined,
            }}
          >
            {it.label}
          </span>
        </li>
      ))}
    </ul>
  );
}

export default App;
