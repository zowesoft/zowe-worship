import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { App } from '../../../bindings/zowe-worship';
import * as models from '../../../bindings/zowe-worship/models';
import { useStudio } from '../../stores/StudioProvider';
import { MicCapture, listAudioInputs, toBase64 } from './mic';
import type { AudioInput } from './mic';

// How much audio one "analyze" request transcribes. Twenty seconds holds a
// spoken reference plus the sentence around it without making the engine
// chew through dead air.
export const ANALYZE_SECONDS = 20;

type DetectionValue = {
  listening: boolean;
  devices: AudioInput[];
  deviceId: string;
  level: number;
  transcript: string;
  candidates: models.DetectionView[];
  analyzing: boolean;
  disabled: boolean;
  engineConfigured: boolean;
  bufferedSeconds: number;
  setDevice: (deviceID: string) => void;
  toggleListening: () => Promise<void>;
  analyze: () => Promise<void>;
  approve: (id: string) => Promise<void>;
  dismiss: (id: string) => Promise<void>;
  clear: () => Promise<void>;
  refresh: () => Promise<void>;
};

const DetectionContext = createContext<DetectionValue | null>(null);

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

export function DetectionProvider({ children }: { children: ReactNode }) {
  const studio = useStudio();
  const [listening, setListening] = useState(false);
  const [devices, setDevices] = useState<AudioInput[]>([]);
  const [deviceId, setDeviceId] = useState('');
  const [level, setLevel] = useState(0);
  const [transcript, setTranscript] = useState('');
  const [candidates, setCandidates] = useState<models.DetectionView[]>([]);
  const [analyzing, setAnalyzing] = useState(false);
  const [disabled, setDisabled] = useState(false);
  const [engineConfigured, setEngineConfigured] = useState(false);
  const [bufferedSeconds, setBufferedSeconds] = useState(0);

  const captureRef = useRef<MicCapture | null>(null);
  const deviceIdRef = useRef('');
  deviceIdRef.current = deviceId;

  const refreshConfig = useCallback(async () => {
    try {
      const config = await App.GetDetectionConfig();
      setDisabled(config.disabled);
      setEngineConfigured(config.engineSpec !== '');
    } catch {
      // Without the bridge the panel stays in its offline state.
    }
  }, []);

  const refreshQueue = useCallback(async () => {
    try {
      setCandidates((await App.ListDetections()) ?? []);
    } catch {
      // Same as above: read-only without the bridge.
    }
  }, []);

  useEffect(() => {
    void refreshConfig();
    void refreshQueue();
  }, [refreshConfig, refreshQueue]);

  // Device labels only become readable once permission is granted, so the
  // list is reread when listening starts.
  useEffect(() => {
    if (!listening) return;
    listAudioInputs().then(setDevices).catch(() => setDevices([]));
  }, [listening]);

  const stopCapture = useCallback(() => {
    captureRef.current?.stop();
    captureRef.current = null;
    setListening(false);
    setLevel(0);
  }, []);

  const startCapture = useCallback(async () => {
    if (captureRef.current?.listening) return;
    const capture = new MicCapture();
    capture.setLevelListener((next) => setLevel(next));
    try {
      await capture.start(deviceIdRef.current || undefined);
    } catch (error) {
      setTranscript('');
      throw new Error(`Microphone unavailable: ${errorMessage(error)}`);
    }
    captureRef.current = capture;
    setListening(true);
  }, []);

  const toggleListening = useCallback(async () => {
    if (listening) {
      stopCapture();
    } else {
      await startCapture();
    }
  }, [listening, startCapture, stopCapture]);

  const analyze = useCallback(async () => {
    const capture = captureRef.current;
    if (!capture || analyzing) return;
    setAnalyzing(true);
    try {
      const pcm = capture.takeLast(ANALYZE_SECONDS);
      setBufferedSeconds(pcm.length / 16000);
      const result = await App.DetectScripture(toBase64(pcm));
      setTranscript(result.transcript);
      setCandidates((await App.ListDetections()) ?? []);
    } finally {
      setAnalyzing(false);
    }
  }, [analyzing]);

  const approve = useCallback(async (id: string) => {
    // Approval previews the passage through the studio, exactly like a manual
    // preview; sending it live stays a separate operator action.
    await studio.approveDetection(id);
    await refreshQueue();
  }, [studio, refreshQueue]);

  const dismiss = useCallback(async (id: string) => {
    setCandidates((await App.DismissDetection(id)) ?? []);
  }, []);

  const clear = useCallback(async () => {
    setCandidates((await App.ClearDetections()) ?? []);
    setTranscript('');
  }, []);

  // Stop the microphone when the studio window goes away, so the device is
  // not held after a reload or close.
  useEffect(() => () => stopCapture(), [stopCapture]);

  const value = useMemo<DetectionValue>(() => ({
    listening,
    devices,
    deviceId,
    level,
    transcript,
    candidates,
    analyzing,
    disabled,
    engineConfigured,
    bufferedSeconds,
    setDevice: setDeviceId,
    toggleListening,
    analyze,
    approve,
    dismiss,
    clear,
    refresh: async () => {
      await Promise.all([refreshConfig(), refreshQueue()]);
    },
  }), [
    listening, devices, deviceId, level, transcript, candidates, analyzing, disabled,
    engineConfigured, bufferedSeconds, toggleListening, analyze, approve, dismiss, clear,
    refreshConfig, refreshQueue,
  ]);

  return <DetectionContext.Provider value={value}>{children}</DetectionContext.Provider>;
}

export function useDetection(): DetectionValue {
  const context = useContext(DetectionContext);
  if (!context) {
    throw new Error('useDetection must be used inside DetectionProvider.');
  }
  return context;
}
