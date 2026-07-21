<script setup>
import { ref, watch, nextTick } from 'vue';
import { playerState } from '../composables/usePlayer.js';

const audioRef = ref(null);

// Setting playerState.src re-renders the :src binding asynchronously;
// wait for that patch before calling play(), mirroring the original's
// synchronous `playerAudio.src = ...; playerAudio.play();`.
watch(
  () => playerState.src,
  async (newSrc) => {
    if (!newSrc) {
      return;
    }
    await nextTick();
    audioRef.value?.play();
  },
);
</script>

<template>
  <div
    v-show="playerState.visible"
    id="player-bar"
    class="fixed bottom-0 inset-x-0 bg-neutral-900 border-t border-neutral-800 px-6 py-3 flex items-center gap-4 z-20"
  >
    <div class="min-w-0">
      <div class="text-sm text-neutral-100 truncate">{{ playerState.title }}</div>
      <div class="text-xs text-neutral-400 truncate">{{ playerState.artist }}</div>
    </div>
    <audio ref="audioRef" :src="playerState.src" controls class="flex-1 min-w-0"></audio>
  </div>
</template>
