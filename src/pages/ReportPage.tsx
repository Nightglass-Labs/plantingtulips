import { useSearchParams } from '@solidjs/router';
import { createSignal, onSettled } from 'solid-js';
import * as stylex from '@stylexjs/stylex';
import { setSeo } from '../lib/seo';
import { s } from '../ui/styles';

export function ReportPage() {
  const [params] = useSearchParams();
  const [state, setState] = createSignal<'idle' | 'sending' | 'sent' | 'error'>('idle');

  onSettled(() => {
    setSeo({
      title: 'Report content · PlantingTulips',
      description: 'PlantingTulips content reporting and safety form.',
      canonicalPath: '/report',
      robots: 'noindex,nofollow,noarchive',
    });
  });

  async function submit(event: SubmitEvent) {
    event.preventDefault();
    setState('sending');
    const form = event.currentTarget as HTMLFormElement;
    const body = Object.fromEntries(new FormData(form).entries());
    const response = await fetch('/api/v1/report', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify(body) });
    setState(response.ok ? 'sent' : 'error');
  }

  return (
    <main {...stylex.props(s.page)}>
      <div {...stylex.props(s.panel)} style={{ 'max-width': '760px' }}>
        <h1 {...stylex.props(s.heading)}>Report content</h1>
        <p {...stylex.props(s.prose)}>Use this form for non-consensual intimate imagery, suspected underage content, impersonation/deepfake concerns, copyright issues, or another urgent safety or legal issue. Reports should be reviewed promptly and a local block can be applied independently of the source provider.</p>
        <form onSubmit={submit} {...stylex.props(s.form)}>
          <label {...stylex.props(s.label)}>Your email<input name="email" type="email" required {...stylex.props(s.input)} /></label>
          <label {...stylex.props(s.label)}>Reason<select name="reason" required {...stylex.props(s.input)}><option value="ncii">Non-consensual intimate imagery</option><option value="minor">Suspected minor</option><option value="deepfake">Impersonation / deepfake</option><option value="copyright">Copyright</option><option value="other">Other</option></select></label>
          <label {...stylex.props(s.label)}>Provider<input name="provider" value={params.provider || ''} {...stylex.props(s.input)} /></label>
          <label {...stylex.props(s.label)}>Video ID<input name="videoId" value={params.id || ''} {...stylex.props(s.input)} /></label>
          <label {...stylex.props(s.label)}>Source URL<input name="sourceUrl" value={params.url || ''} {...stylex.props(s.input)} /></label>
          <label {...stylex.props(s.label)}>Details<textarea name="details" required {...stylex.props(s.textarea)} /></label>
          <button disabled={state() === 'sending'} {...stylex.props(s.primaryButton)}>{state() === 'sending' ? 'Sending…' : 'Submit report'}</button>
          {state() === 'sent' && <p>Report received.</p>}
          {state() === 'error' && <p>Report could not be submitted. Preserve the report details and contact the operator directly.</p>}
        </form>
      </div>
    </main>
  );
}
