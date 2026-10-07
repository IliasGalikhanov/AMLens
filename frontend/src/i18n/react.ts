import { useSyncExternalStore } from 'react';
import { getLocale, subscribe } from './core.ts';

export function useLocale() {
  return useSyncExternalStore(subscribe, getLocale, () => 'en' as const);
}
