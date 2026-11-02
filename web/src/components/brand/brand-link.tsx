import Link from 'next/link';

import styles from './brand-link.module.css';
import { BrandMark } from './brand-mark';

interface Props {
  size: 24 | 28;
}

export function BrandLink({ size }: Props) {
  return (
    <Link href="/" className={size === 28 ? styles.large : styles.small}>
      <BrandMark size={size} />
      Gitorgasm
    </Link>
  );
}
