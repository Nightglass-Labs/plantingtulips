import { useNavigate, type RouteSectionProps } from '@solidjs/router';
import { Show, createSignal, onSettled } from 'solid-js';
import * as stylex from '@stylexjs/stylex';
import { s } from './styles';

export function AppShell(props: RouteSectionProps) {
  const navigate = useNavigate();
  const [q, setQ] = createSignal('');
  const [accepted, setAccepted] = createSignal(true);

  onSettled(() => {
    setAccepted(localStorage.getItem('pt-age-ack') === '1');
  });

  const accept = () => {
    localStorage.setItem('pt-age-ack', '1');
    setAccepted(true);
  };

  return (
    <div {...stylex.props(s.app)}>
      <header {...stylex.props(s.header)}>
        <a href="/" {...stylex.props(s.brand)}>planting<span {...stylex.props(s.brandAccent)}>tulips</span></a>
        <form {...stylex.props(s.searchWrap)} onSubmit={(event) => { event.preventDefault(); navigate(`/?q=${encodeURIComponent(q() || 'blowjob')}`); }}>
          <input value={q()} onInput={(event) => setQ(event.currentTarget.value)} placeholder="Search the catalog" aria-label="Search videos" {...stylex.props(s.searchInput)} />
        </form>
        <nav {...stylex.props(s.nav)}><a href="/categories" {...stylex.props(s.navLink)}>Browse</a><a href="/library" {...stylex.props(s.navLink)}>Library</a><a href="/report" {...stylex.props(s.navLink)}>Report</a></nav>
      </header>

      {props.children}

      <footer {...stylex.props(s.footer)}>
        <div {...stylex.props(s.footerInner)}>
          <span>PlantingTulips · discovery and embeds, not re-hosted media</span>
          <div {...stylex.props(s.footerLinks)}><a href="/library">Library</a><a href="/terms">Terms</a><a href="/privacy">Privacy</a><a href="/dmca">DMCA</a><a href="/2257">2257</a><a href="/report">Report content</a></div>
        </div>
      </footer>

      <Show when={!accepted()}>
        <div {...stylex.props(s.gate)}><div {...stylex.props(s.gateCard)}><h1 {...stylex.props(s.gateTitle)}>Adults only</h1><p {...stylex.props(s.gateText)}>This site is intended only for adults. By continuing, you confirm that you are at least 18 and legally permitted to view adult material where you are located. This acknowledgement is not a substitute for any jurisdiction-specific age-verification requirement.</p><div {...stylex.props(s.buttonRow)}><button type="button" onClick={accept} {...stylex.props(s.primaryButton)}>I am 18+</button><button type="button" onClick={() => history.back()} {...stylex.props(s.secondaryButton)}>Leave</button></div></div></div>
      </Show>
    </div>
  );
}
