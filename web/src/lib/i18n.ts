export const primitiveMessages = { close: 'Close', loading: 'Loading…', details: 'Technical details' } as const;
export type MessageKey = keyof typeof primitiveMessages;
export function t(key: MessageKey): string { return primitiveMessages[key]; }
