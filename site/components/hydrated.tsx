'use client';
import { useEffect } from 'react';

/** Marks the page as hydrated; the failsafe in the root layout reads the flag. */
export function Hydrated() {
  useEffect(() => {
    (window as { __nessHydrated?: boolean }).__nessHydrated = true;
  }, []);
  return null;
}
