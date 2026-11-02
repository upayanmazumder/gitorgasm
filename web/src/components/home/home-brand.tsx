import Link from 'next/link';

import { BrandMark } from '@/components/brand';

import styles from './home.module.css';

interface Props {
  size: 24 | 28;
}

export function HomeBrand({ size }: Props) {
  return (
    <Link href="/" className={size === 28 ? styles.brandLarge : styles.brandSmall}>
      <BrandMark size={size} />
      Gitorgasm
    </Link>
  );
}
