<script setup>
import { ref, watch, nextTick } from 'vue';
import { playerState, closePlayer } from '../composables/usePlayer.js';

const audioRef = ref(null);

// Watches playToken rather than src: replaying the same track leaves src
// unchanged, so a watcher on src alone wouldn't fire (Vue's watch() only
// fires on a value CHANGE), silently failing to (re)start playback. Setting
// playerState.src re-renders the :src binding asynchronously; wait for that
// patch before calling play(), mirroring the original's synchronous
// `playerAudio.src = ...; playerAudio.play();` — which, since assigning
// .src always reloads a media element even when the value is unchanged,
// also always restarted from 0. currentTime is reset here to match.
watch(
  () => playerState.playToken,
  async () => {
    if (!playerState.src) {
      return;
    }
    await nextTick();
    if (audioRef.value) {
      audioRef.value.currentTime = 0;
    }
    audioRef.value?.play();
  },
);

function onClose() {
  audioRef.value?.pause();
  closePlayer();
}
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
    <button class="text-neutral-400 hover:text-neutral-100 text-xl leading-none" title="Close player" @click="onClose">&times;</button>
  </div>
</template>
