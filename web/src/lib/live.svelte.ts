export type CollectorState = 'healthy' | 'degraded' | 'down' | 'unknown';
export interface LiveSnapshot {
  seq: number;
  ts: string;
  bootIdentity: string;
  host: Record<string, number | null>;
  resources: Array<{
    id: string;
    name: string;
    status: string;
    cpuHostPct?: number | null;
    memoryBytes?: number | null;
  }>;
  collectors: Record<
    string,
    { state: CollectorState; reason?: string; freshAt?: string }
  >;
}
export interface LiveEvent {
  id: number;
  type: string;
  message: string;
  resourceId?: string;
}
export type ConnectionState =
  'connecting' | 'connected' | 'disconnected' | 'unauthorized';

export class LiveStore {
  snapshot = $state<LiveSnapshot | null>(null);
  events = $state<LiveEvent[]>([]);
  state = $state<ConnectionState>('disconnected');
  private source: EventSource | null = null;
  private attempts = 0;
  connect(url = '/api/v1/live') {
    this.close();
    this.state = 'connecting';
    const source = new EventSource(url);
    this.source = source;
    source.addEventListener('snapshot', (event) => {
      this.snapshot = JSON.parse((event as MessageEvent).data) as LiveSnapshot;
      this.state = 'connected';
      this.attempts = 0;
    });
    source.addEventListener('event', (event) => {
      const value = JSON.parse((event as MessageEvent).data) as LiveEvent;
      if (!this.events.some((item) => item.id === value.id))
        this.events = [...this.events.slice(-127), value];
    });
    source.onerror = () => {
      if (this.source !== source) return;
      this.state = 'disconnected';
      source.close();
      const delay = Math.min(1000 * 2 ** this.attempts++, 30000);
      window.setTimeout(() => this.connect(url), delay);
    };
  }
  close() {
    this.source?.close();
    this.source = null;
  }
}
export async function sessionActive(fetcher = fetch): Promise<boolean> {
  const response = await fetcher('/api/v1/session', {
    credentials: 'same-origin',
  });
  return response.status === 204;
}
