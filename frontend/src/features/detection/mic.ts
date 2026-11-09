// Mic capture for Live Detection: getUserMedia → AudioWorklet → 16 kHz mono
// Int16 PCM ring buffer, the WAV-ready format the engine expects.

const TARGET_RATE = 16000;
// Sixty seconds covers a slow reference plus the sentence around it.
const BUFFER_SECONDS = 60;

// The worklet converts float frames to Int16 PCM, decimating by the ratio
// between the real sample rate and 16 kHz when the context could not open at it.
const processorSource = `
class ZoweCaptureProcessor extends AudioWorkletProcessor {
  constructor() {
    super();
    // Input samples per output sample: 1 at the target rate, 3 at 48 kHz.
    this.ratio = sampleRate / ${TARGET_RATE};
    // Fractional read cursor carried between blocks. The cursor is measured
    // within each 128-sample block and the overshoot rolled into the next one;
    // an absolute counter would freeze after the first block and the rest of the
    // capture would be silence.
    this.cursor = 0;
  }
  process(inputs) {
    const channel = inputs[0] && inputs[0][0];
    if (channel) {
      const count = this.cursor < channel.length
        ? Math.max(0, Math.ceil((channel.length - this.cursor) / this.ratio))
        : 0;
      if (count > 0) {
        const out = new Int16Array(count);
        let pos = this.cursor;
        for (let n = 0; n < count; n++) {
          const sample = channel[Math.min(channel.length - 1, Math.round(pos))] ?? 0;
          const s = Math.max(-1, Math.min(1, sample));
          out[n] = s < 0 ? s * 0x8000 : s * 0x7fff;
          pos += this.ratio;
        }
        this.cursor = pos - channel.length;
        this.port.postMessage(out.buffer, [out.buffer]);
      }
    }
    return true;
  }
}
registerProcessor('zowe-capture', ZoweCaptureProcessor);
`;

export type AudioInput = {
  deviceId: string;
  label: string;
};

class RingBuffer {
  private readonly samples: Int16Array;
  private writePos = 0;
  private filled = 0;

  constructor(capacity: number) {
    this.samples = new Int16Array(capacity);
  }

  append(chunk: Int16Array) {
    const capacity = this.samples.length;
    for (let i = 0; i < chunk.length; i++) {
      this.samples[this.writePos] = chunk[i];
      this.writePos = (this.writePos + 1) % capacity;
    }
    this.filled = Math.min(capacity, this.filled + chunk.length);
  }

  takeLast(count: number): Int16Array {
    const available = Math.min(count, this.filled);
    const result = new Int16Array(available);
    const capacity = this.samples.length;
    let start = (this.writePos - available + capacity) % capacity;
    for (let i = 0; i < available; i++) {
      result[i] = this.samples[(start + i) % capacity];
    }
    return result;
  }

  get seconds(): number {
    return this.filled / TARGET_RATE;
  }
}

export class MicCapture {
  private stream: MediaStream | null = null;
  private context: AudioContext | null = null;
  private worklet: AudioWorkletNode | null = null;
  private source: MediaStreamAudioSourceNode | null = null;
  private buffer = new RingBuffer(BUFFER_SECONDS * TARGET_RATE);
  private onLevel: ((level: number, bufferedSeconds: number) => void) | null = null;

  get listening(): boolean {
    return this.stream !== null;
  }

  get bufferedSeconds(): number {
    return this.buffer.seconds;
  }

  setLevelListener(listener: (level: number, bufferedSeconds: number) => void) {
    this.onLevel = listener;
  }

  async start(deviceId?: string): Promise<void> {
    if (this.stream) return;

    const constraints: MediaStreamConstraints = {
      audio: {
        deviceId: deviceId ? { exact: deviceId } : undefined,
        // ASR wants the room as it sounds, not the browser's cleanup pass.
        echoCancellation: false,
        noiseSuppression: false,
        autoGainControl: false,
        channelCount: 1,
      },
    };
    const stream = await navigator.mediaDevices.getUserMedia(constraints);

    const context = new AudioContext({ sampleRate: TARGET_RATE });
    try {
      const blobURL = URL.createObjectURL(new Blob([processorSource], { type: 'application/javascript' }));
      await context.audioWorklet.addModule(blobURL);
      URL.revokeObjectURL(blobURL);
    } catch (error) {
      void context.close();
      for (const track of stream.getTracks()) track.stop();
      throw error;
    }

    const worklet = new AudioWorkletNode(context, 'zowe-capture');
    worklet.port.onmessage = (event) => {
      const chunk = new Int16Array(event.data as ArrayBuffer);
      this.buffer.append(chunk);
      if (this.onLevel) {
        // Report level and buffered seconds from the same tick, so the readout
        // moves while listening instead of sitting at zero until the first
        // analyze.
        this.onLevel(rms(chunk), this.buffer.seconds);
      }
    };

    this.source = context.createMediaStreamSource(stream);
    this.source.connect(worklet);
    // The worklet is a sink, not a source: nothing is played back.
    this.context = context;
    this.stream = stream;
    this.worklet = worklet;
  }

  stop() {
    this.stream?.getTracks().forEach((track) => track.stop());
    void this.worklet?.disconnect();
    void this.source?.disconnect();
    void this.context?.close();
    this.stream = null;
    this.context = null;
    this.worklet = null;
    this.source = null;
    this.onLevel?.(0, this.buffer.seconds);
  }

  takeLast(seconds: number): Int16Array {
    return this.buffer.takeLast(Math.round(seconds * TARGET_RATE));
  }

  clear() {
    this.buffer = new RingBuffer(BUFFER_SECONDS * TARGET_RATE);
  }
}

function rms(chunk: Int16Array): number {
  if (chunk.length === 0) return 0;
  let total = 0;
  for (let i = 0; i < chunk.length; i++) {
    total += chunk[i] * chunk[i];
  }
  return Math.min(1, Math.sqrt(total / chunk.length) / 32768 * 4);
}

export async function listAudioInputs(): Promise<AudioInput[]> {
  if (!navigator.mediaDevices?.enumerateDevices) return [];
  const devices = await navigator.mediaDevices.enumerateDevices();
  return devices
    .filter((device) => device.kind === 'audioinput')
    .map((device) => ({ deviceId: device.deviceId, label: device.label || 'Microphone' }));
}

// toBase64 encodes PCM bytes for the Wails bridge, which delivers []byte
// bindings as base64 strings.
export function toBase64(pcm: Int16Array): string {
  const bytes = new Uint8Array(pcm.buffer, pcm.byteOffset, pcm.byteLength);
  let binary = '';
  const chunkSize = 0x8000;
  for (let i = 0; i < bytes.length; i += chunkSize) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunkSize));
  }
  return btoa(binary);
}
