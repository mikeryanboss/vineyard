import chokidar, { type FSWatcher } from "chokidar";
import type { FileWatcher } from "./types.js";

export class ChokidarFileWatcher implements FileWatcher {
  private watcher: FSWatcher | undefined;

  async start(paths: string[], onChange: (path: string) => void): Promise<void> {
    await this.stop();
    this.watcher = chokidar.watch(paths, {
      ignoreInitial: true,
      awaitWriteFinish: {
        stabilityThreshold: 150,
        pollInterval: 25,
      },
    });
    this.watcher.on("add", onChange);
    this.watcher.on("change", onChange);
    this.watcher.on("unlink", onChange);
  }

  async add(paths: string[]): Promise<void> {
    this.watcher?.add(paths);
  }

  async stop(): Promise<void> {
    if (!this.watcher) return;
    const watcher = this.watcher;
    this.watcher = undefined;
    await watcher.close();
  }
}
