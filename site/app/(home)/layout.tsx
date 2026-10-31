import { HomeLayout } from 'fumadocs-ui/layouts/home';
import { baseOptions, homeLinks } from '@/lib/layout.shared';
import { Footer } from '@/components/footer';

export default function Layout({ children }: LayoutProps<'/'>) {
  return (
    <HomeLayout {...baseOptions()} links={homeLinks} nav={{ ...baseOptions().nav, transparentMode: 'top' }}>
      {children}
      <Footer />
    </HomeLayout>
  );
}
