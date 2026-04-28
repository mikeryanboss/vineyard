import { EventEmitter } from "node:events";
import type { EventBus, EventMap, OrchestratorEventName } from "./types.js";

export function createEventBus(): EventBus {
  const emitter = new EventEmitter();
  emitter.on("error", () => undefined);

  return {
    emit<K extends OrchestratorEventName>(event: K, data: EventMap[K]): void {
      emitter.emit(event, data);
    },

    on<K extends OrchestratorEventName>(event: K, handler: (data: EventMap[K]) => void | Promise<void>): () => void {
      const safeHandler = async (data: EventMap[K]) => {
        try {
          await handler(data);
        } catch (error) {
          emitter.emit("error", {
            error: error instanceof Error ? error : new Error(String(error)),
          } satisfies EventMap["error"]);
        }
      };
      emitter.on(event, safeHandler);
      return () => emitter.off(event, safeHandler);
    },

    clear(): void {
      emitter.removeAllListeners();
    },
  };
}
