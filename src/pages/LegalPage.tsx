import { useParams } from '@solidjs/router';
import { createEffect } from 'solid-js';
import * as stylex from '@stylexjs/stylex';
import { setSeo } from '../lib/seo';
import { s } from '../ui/styles';

const docs: Record<string, { title: string; body: string[] }> = {
  terms: { title: 'Terms', body: ['PlantingTulips is a discovery service for third-party adult media. Users must be adults and comply with applicable law.', "Third-party embeds and metadata remain subject to their source provider's terms. Content may be removed or blocked from the local catalog at any time."] },
  privacy: { title: 'Privacy', body: ['Keep analytics minimal and avoid collecting sensitive viewing history unless it is necessary and disclosed.', 'Before production launch, replace this placeholder with the final privacy notice covering analytics, advertising, age-verification vendors, retention, and user rights.'] },
  dmca: { title: 'DMCA', body: ['Copyright owners can use the Report Content workflow to identify the specific indexed or embedded item and provide the information required for a valid notice.', "Before launch, designate the operator's DMCA agent and publish the required contact information."] },
  '2257': { title: '18 U.S.C. § 2257 notice', body: ['PlantingTulips is designed as an index/embed service and does not produce or re-host source video files.', "Do not rely on this placeholder as legal advice. Before production launch, have counsel review the operator's recordkeeping obligations and the notices supplied by each content provider."] },
};

export function LegalPage() {
  const params = useParams();
  const key = () => params.document ?? '';
  const doc = () => docs[key()] ?? { title: 'Not found', body: ['This page does not exist.'] };

  createEffect(key, (value) => {
    const current = docs[value];
    setSeo(current ? {
      title: `${current.title} · PlantingTulips`,
      description: `${current.title} information for PlantingTulips.`,
      canonicalPath: `/${value}`,
      robots: 'index,follow',
    } : {
      title: 'Not found · PlantingTulips',
      description: 'This PlantingTulips page does not exist.',
      canonicalPath: `/${value}`,
      robots: 'noindex,follow',
    });
  });

  return <main {...stylex.props(s.page)}><article {...stylex.props(s.panel, s.prose)}><h1 {...stylex.props(s.heading)}>{doc().title}</h1>{doc().body.map((paragraph) => <p>{paragraph}</p>)}</article></main>;
}
