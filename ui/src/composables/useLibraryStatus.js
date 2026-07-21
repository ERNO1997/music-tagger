import { reactive } from 'vue';

// The shared "#status" line above the table/grid views — one instance,
// updated by loadLibrary() and by job trigger/poll error paths, exactly
// like the original's single `#status` element.
export const libraryStatus = reactive({
  text: 'Loading tracked files…',
  isError: false,
});
