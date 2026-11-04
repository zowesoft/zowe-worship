// File-based audio for testing Live Detection without a microphone: decode
// any format the webview supports (MP3, M4A, WAV, OGG…) and resample it to
// the 16 kHz mono Int16 PCM the Go pipeline expects.

// Cap how much of a file gets analyzed: the engine runs on CPU and a full
// sermon would take minutes per chunk. Long enough for any generated clip.
const MAX_FILE_SECONDS = 300;
const TARGET_RATE = 16000;

export async function decodeAudioFile(file: File | Blob): Promise<Int16Array> {
  const bytes = await file.arrayBuffer();
  const context = new AudioContext();
  let decoded: AudioBuffer;
  try {
    decoded = await context.decodeAudioData(bytes);
  } finally {
    void context.close();
  }

  const seconds = Math.min(decoded.duration, MAX_FILE_SECONDS);
  const frames = Math.max(1, Math.ceil(seconds * TARGET_RATE));
  const offline = new OfflineAudioContext(1, frames, TARGET_RATE);
  const source = offline.createBufferSource();
  source.buffer = decoded;
  source.connect(offline.destination);
  source.start(0);
  const rendered = await offline.startRendering();

  const mono = rendered.getChannelData(0);
  const pcm = new Int16Array(mono.length);
  for (let i = 0; i < mono.length; i++) {
    const s = Math.max(-1, Math.min(1, mono[i]));
    pcm[i] = s < 0 ? s * 0x8000 : s * 0x7fff;
  }
  return pcm;
}
