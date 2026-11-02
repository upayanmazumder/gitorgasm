import { Bricolage_Grotesque, Instrument_Sans } from 'next/font/google';

// Scoped to the homepage root so application screens keep the root layout fonts.
export const displayFont = Bricolage_Grotesque({
  subsets: ['latin'],
  axes: ['opsz'],
  variable: '--font-home-display',
  display: 'swap',
});

export const textFont = Instrument_Sans({
  subsets: ['latin'],
  variable: '--font-home-text',
  display: 'swap',
});
