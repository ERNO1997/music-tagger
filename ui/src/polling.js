// Generic poll-until-not-running helper shared by scan/identify/enrich/tag/
// relocate. Calling start() while already running is a no-op.
export function pollJob({ fetchStatus, onUpdate, intervalMs = 1000 }) {
  let timer = null;
  return {
    start() {
      if (timer) {
        return;
      }
      timer = setInterval(async () => {
        try {
          const status = await fetchStatus();
          await onUpdate(status);
          if (!status.running) {
            clearInterval(timer);
            timer = null;
          }
        } catch (err) {
          clearInterval(timer);
          timer = null;
        }
      }, intervalMs);
    },
  };
}
