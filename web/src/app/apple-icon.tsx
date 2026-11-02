import { ImageResponse } from 'next/og';

import {
  MARK_BODY,
  MARK_EARS,
  MARK_GLYPH_TRANSFORM,
  MARK_HEAD,
  MARK_RING,
  MARK_TAIL,
} from '@/components/brand';

export const size = { width: 180, height: 180 };
export const contentType = 'image/png';

const FUR = '#F4B02A';
const INK = '#14202E';

const cat = [
  '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="124" height="124">',
  `<g fill="${INK}" transform="${MARK_GLYPH_TRANSFORM}">`,
  ...MARK_EARS.map(d => `<path d="${d}"/>`),
  `<path fill-rule="evenodd" d="${MARK_HEAD}"/>`,
  `<path d="${MARK_BODY}"/></g>`,
  `<g fill="none" stroke="${INK}" stroke-linecap="round" transform="${MARK_GLYPH_TRANSFORM}">`,
  `<path d="${MARK_TAIL}" stroke-width="2.4"/>`,
  `<circle cx="${MARK_RING.cx}" cy="${MARK_RING.cy}" r="${MARK_RING.r}" stroke-width="2.2"/></g>`,
  '</svg>',
].join('');

export default function AppleIcon() {
  const src = `data:image/svg+xml;base64,${Buffer.from(cat).toString('base64')}`;

  return new ImageResponse(
    <div
      style={{
        width: '100%',
        height: '100%',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: FUR,
      }}
    >
      <img src={src} width={124} height={124} alt="" />
    </div>,
    size
  );
}
