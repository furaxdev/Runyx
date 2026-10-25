// A per-frame registry of clickable/hoverable rectangles. Widgets
// register a zone when they draw; the app's mouse handler hit-tests
// against this list (reverse order = topmost first) instead of each
// widget tracking its own screen position across frames.
import type { Rect } from "../layout/flex.js";

export interface Zone<T = unknown> {
  id: string;
  rect: Rect;
  kind: string;
  data: T;
}

export class ZoneRegistry {
  private zones: Zone[] = [];

  reset(): void {
    this.zones = [];
  }

  register<T>(id: string, rect: Rect, kind: string, data: T): void {
    this.zones.push({ id, rect, kind, data });
  }

  /** Returns the topmost (last-registered) zone containing (row, col), if any. */
  hitTest(row: number, col: number): Zone | null {
    for (let i = this.zones.length - 1; i >= 0; i--) {
      const z = this.zones[i]!;
      if (row >= z.rect.row && row < z.rect.row + z.rect.height && col >= z.rect.col && col < z.rect.col + z.rect.width) {
        return z;
      }
    }
    return null;
  }

  all(): readonly Zone[] {
    return this.zones;
  }
}

export function rectContains(r: Rect, row: number, col: number): boolean {
  return row >= r.row && row < r.row + r.height && col >= r.col && col < r.col + r.width;
}
