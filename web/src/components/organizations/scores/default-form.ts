import type { ScoreForm } from './types';

export const defaultForm: ScoreForm = {
  period: 'last30_days',
  start: '',
  end: '',
  category: 'all',
  search: '',
  repository: '',
  userId: '',
  kinds: '',
  excludeBots: true,
  rules: '',
};
