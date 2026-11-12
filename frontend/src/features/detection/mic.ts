// The microphone belongs to the Go AudioService: it captures over WASAPI and
// hands 16 kHz mono PCM to whoever asks for it. What is left in the webview is
// the device list the panel shows and the base64 encoder the bridge needs for
// the PCM the file decoder produces.

import { App } from '../../../bindings/zopresent';

export type AudioInput = {
  deviceId: string;
  label: string;
};

// listAudioInputs reads the inputs the backend can open. The names come from
// the audio stack itself, so they are there without a permission prompt.
export async function listAudioInputs(): Promise<AudioInput[]> {
  return (await App.AudioInputs()) ?? [];
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
