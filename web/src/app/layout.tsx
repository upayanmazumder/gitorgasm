import type { Metadata } from 'next';
import { NuqsAdapter } from 'nuqs/adapters/next/app';

import './globals.css';

import { MotionProvider } from '@/components/motion';
import { QueryProvider, ThemeProvider } from '@/components/providers';
import { Toaster } from '@/components/ui';
import { cn } from '@/lib';
import { fontVariables } from '@/styles';

export const metadata: Metadata = {
  title: 'Gitorgasm',
  description: 'Gitorgasm web app',
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning className={cn('h-full', 'antialiased', fontVariables)}>
      {/* Browser extensions add attributes to body before hydration. The suppression covers only body's own attributes. */}
      <body suppressHydrationWarning className="flex min-h-full flex-col">
        <ThemeProvider
          attribute="class"
          defaultTheme="system"
          enableSystem
          disableTransitionOnChange
        >
          <QueryProvider>
            <NuqsAdapter>
              <MotionProvider>{children}</MotionProvider>
            </NuqsAdapter>
          </QueryProvider>
          <Toaster />
        </ThemeProvider>
      </body>
    </html>
  );
}
