import { useState } from 'react';
import type { DragEvent } from 'react';

type RowHandlers = {
  draggable: boolean;
  onDragStart: (event: DragEvent<HTMLElement>) => void;
  onDragOver: (event: DragEvent<HTMLElement>) => void;
  onDrop: (event: DragEvent<HTMLElement>) => void;
  onDragEnd: () => void;
};

// moveWithin returns the ids with the entry at `from` reinserted at `to`, which
// is how a dropped row lands: the rows it passed shift over rather than swap.
export function moveWithin(ids: string[], from: number, to: number): string[] {
  const next = [...ids];
  const [moved] = next.splice(from, 1);
  next.splice(to, 0, moved);
  return next;
}

export function useDragOrder(commit: (from: number, to: number) => void): {
  dragging: number | null;
  over: number | null;
  rowProps: (index: number) => RowHandlers;
} {
  const [dragging, setDragging] = useState<number | null>(null);
  const [over, setOver] = useState<number | null>(null);

  function reset() {
    setDragging(null);
    setOver(null);
  }

  return {
    dragging,
    over,
    rowProps: (index) => ({
      draggable: true,
      onDragStart: (event) => {
        event.dataTransfer.effectAllowed = 'move';
        setDragging(index);
      },
      onDragOver: (event) => {
        if (dragging === null) return;
        event.preventDefault();
        event.dataTransfer.dropEffect = 'move';
        if (over !== index) setOver(index);
      },
      onDrop: (event) => {
        event.preventDefault();
        if (dragging !== null && dragging !== index) commit(dragging, index);
        reset();
      },
      onDragEnd: reset,
    }),
  };
}
