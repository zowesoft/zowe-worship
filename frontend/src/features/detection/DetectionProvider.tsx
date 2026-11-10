import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { Events } from '@wailsio/runtime';
import { App } from '../../../bindings/zowe-worship';
import * as models from '../../../bindings/zowe-worship/models';
import { useStudio } from '../../stores/StudioProvider';
import { MicCapture, listAudioInputs, toBase64 } from './mic';
import type { AudioInput } from './mic';
import { decodeAudioFile } from './audioFile';
import { SpeechSegmenter } from './segmenter';

// How much audio one analyze request transcribes: a spoken reference plus the
// sentence around it, without making the engine chew through dead air.
export const ANALYZE_SECONDS = 20;

type DetectionValue = {
  listening: boolean;
  devices: AudioInput[];
  deviceId: string;
  level: number;
  transcript: string;
  candidates: models.DetectionView[];
  // Decisions already taken, newest first: the panel's recent list.
  history: models.DetectionHistoryView[];
  analyzing: boolean;
  disabled: boolean;
  engineConfigured: boolean;
  bufferedSeconds: number;
  // The last analysis segment and how many repeated hearings it folded into a
  // pending card instead of queuing duplicates.
  segment: number;
  repeats: number;
  // The operator's standing auto-projection choice and the F9 pause. Both are
  // backend policy state; the panel never decides a gate itself.
  autoProjection: boolean;
  autoProjectionPaused: boolean;
  // References the safety gates projected during the last analysis.
  autoProjected: string[];
  // Continuous listening closes a segment when the speaker pauses instead of
  // waiting for the operator to press Analyze; segmenting reports live speech.
  continuous: boolean;
  segmenting: boolean;
  lastError: string | null;
  toggleContinuous: () => void;
  setDevice: (deviceID: string) => void;
  setAutoProjection: (enabled: boolean) => Promise<void>;
  toggleAutoProjectionPause: () => Promise<void>;
  toggleListening: () => Promise<void>;
  analyze: () => Promise<void>;
  analyzeFile: (file: File) => Promise<void>;
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
  const [history, setHistory] = useState<models.DetectionHistoryView[]>([]);
  const [analyzing, setAnalyzing] = useState(false);
  const [disabled, setDisabled] = useState(false);
  const [engineConfigured, setEngineConfigured] = useState(false);
  const [bufferedSeconds, setBufferedSeconds] = useState(0);
  const [segment, setSegment] = useState(0);
  const [repeats, setRepeats] = useState(0);
  const [autoProjection, setAutoProjectionState] = useState(false);
  const [autoProjectionPaused, setAutoProjectionPausedState] = useState(true);
  const [autoProjected, setAutoProjected] = useState<string[]>([]);
  const [continuous, setContinuous] = useState(true);
  const [segmenting, setSegmenting] = useState(false);
  const [lastError, setLastError] = useState<string | null>(null);

  const captureRef = useRef<MicCapture | null>(null);
  const deviceIdRef = useRef('');
  deviceIdRef.current = deviceId;

  // Refs, not state: the capture callback and the segment timer both read these
  // between renders, and a stale state copy would drift from the audio.
  const levelRef = useRef(0);
  const zeroCrossingRef = useRef(0);
  const segmenterRef = useRef(new SpeechSegmenter());
  const busyRef = useRef(false);
  const queuedRef = useRef<Int16Array | null>(null);

  const refreshConfig = useCallback(async () => {
    try {
      const config = await App.GetDetectionConfig();
      setDisabled(config.disabled);
      setEngineConfigured(config.engineSpec !== '');
      setAutoProjectionState(config.autoProjection);
      setAutoProjectionPausedState(config.autoProjectionPaused);
    } catch {
      // Without the bridge the panel stays in its offline state.
    }
  }, []);

  const refreshQueue = useCallback(async () => {
    try {
      setCandidates((await App.ListDetections()) ?? []);
    } catch {
      // Read-only without the bridge.
    }
  }, []);

  // The queue drops a card the moment it is decided, so the panel's recent list
  // comes from detection_history rather than from memory.
  const refreshHistory = useCallback(async () => {
    try {
      setHistory((await App.ListDetectionHistory(20)) ?? []);
    } catch {
      // Read-only without the bridge.
    }
  }, []);

  useEffect(() => {
    void refreshConfig();
    void refreshQueue();
    void refreshHistory();
  }, [refreshConfig, refreshQueue, refreshHistory]);

  // The backend streams the transcript while the engine runs, so the panel fills
  // segment by segment instead of all at once.
  useEffect(() => {
    const cancel = Events.On('detection:progress', (event) => {
      const progress = event.data as { transcript: string; candidates: models.DetectionView[]; segment: number };
      setTranscript(progress.transcript);
      setCandidates(progress.candidates);
      setSegment(progress.segment);
    });
    return () => { cancel(); };
  }, []);

  // Settings is a separate window, so config arrives as an event; re-read it or
  // the studio keeps showing "No engine" after the operator chose one.
  useEffect(() => {
    const cancel = Events.On('detection:config', () => { void refreshConfig(); });
    return () => { cancel(); };
  }, [refreshConfig]);

  // Device labels only become readable once permission is granted, so the
  // list is reread when listening starts.
  useEffect(() => {
    if (!listening) return;
    listAudioInputs().then(setDevices).catch(() => setDevices([]));
  }, [listening]);

  const stopCapture = useCallback(() => {
    captureRef.current?.stop();
    captureRef.current = null;
    segmenterRef.current.reset();
    queuedRef.current = null;
    setSegmenting(false);
    setListening(false);
    setLevel(0);
  }, []);

  const startCapture = useCallback(async () => {
    if (captureRef.current?.listening) return;
    const capture = new MicCapture();
    // The level callback fires per audio block, which doubles as the clock for
    // the buffered-seconds readout.
    capture.setLevelListener((features, seconds) => {
      levelRef.current = features.level;
      zeroCrossingRef.current = features.zeroCrossingRate;
      setLevel(features.level);
      setBufferedSeconds(seconds);
    });
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
      setRepeats(0);
      setAutoProjected([]);
      const result = await App.DetectScripture(toBase64(pcm));
      setTranscript(result.transcript);
      setSegment(result.segment);
      setRepeats(result.repeats);
      setAutoProjected(result.autoProjected ?? []);
      setCandidates((await App.ListDetections()) ?? []);
    } finally {
      setAnalyzing(false);
    }
  }, [analyzing]);

  // A recording goes through the same pipeline, so detection can be tried
  // without a microphone.
  const analyzeFile = useCallback(async (file: File) => {
    if (analyzing) return;
    setAnalyzing(true);
    try {
      const pcm = await decodeAudioFile(file);
      setBufferedSeconds(pcm.length / 16000);
      setRepeats(0);
      setAutoProjected([]);
      const result = await App.DetectScripture(toBase64(pcm));
      setTranscript(result.transcript);
      setSegment(result.segment);
      setRepeats(result.repeats);
      setAutoProjected(result.autoProjected ?? []);
      setCandidates((await App.ListDetections()) ?? []);
    } finally {
      setAnalyzing(false);
    }
  }, [analyzing]);

  // One analysis at a time: a segment that closes mid-transcription waits in a
  // single slot, so a fast speaker cannot grow an unbounded request queue.
  const analyzeSegment = useCallback(async (pcm: Int16Array) => {
    if (pcm.length === 0) return;
    if (busyRef.current) {
      queuedRef.current = pcm;
      return;
    }
    busyRef.current = true;
    setAnalyzing(true);
    setLastError(null);
    let next: Int16Array | null = pcm;
    try {
      while (next) {
        setBufferedSeconds(next.length / 16000);
        const result = await App.DetectScripture(toBase64(next));
        setTranscript(result.transcript);
        setSegment(result.segment);
        setRepeats(result.repeats);
        setAutoProjected(result.autoProjected ?? []);
        setCandidates((await App.ListDetections()) ?? []);
        next = queuedRef.current;
        queuedRef.current = null;
      }
    } catch (error) {
      // The loop has no caller to throw to, so the panel shows the reason.
      setLastError(errorMessage(error));
    } finally {
      busyRef.current = false;
      setAnalyzing(false);
    }
  }, []);

  const toggleContinuous = useCallback(() => setContinuous((value) => !value), []);

  // Continuous mode: watch the level the capture already reports and transcribe
  // each segment as the speaker pauses. The microphone keeps running through a
  // whole sermon; one request goes out per reference, not per 20 seconds.
  useEffect(() => {
    if (!listening || !continuous || disabled) {
      segmenterRef.current.reset();
      setSegmenting(false);
      return;
    }
    segmenterRef.current.reset();
    const timer = window.setInterval(() => {
      const capture = captureRef.current;
      if (!capture) return;
      const range = segmenterRef.current.feed(
        { level: levelRef.current, zeroCrossingRate: zeroCrossingRef.current },
        capture.capturedSamples,
      );
      setSegmenting(segmenterRef.current.hearing);
      if (range) void analyzeSegment(capture.takeRange(range.from, range.to));
    }, 120);
    return () => window.clearInterval(timer);
  }, [listening, continuous, disabled, analyzeSegment]);

  const setAutoProjectionEnabled = useCallback(async (enabled: boolean) => {
    await App.SetAutoProjection(enabled);
    await refreshConfig();
  }, [refreshConfig]);

  // F9 pauses or resumes; it never changes the standing switch.
  const toggleAutoProjectionPause = useCallback(async () => {
    await App.SetAutoProjectionPaused(!autoProjectionPaused);
    await refreshConfig();
  }, [autoProjectionPaused, refreshConfig]);

  const approve = useCallback(async (id: string) => {
    // Approval previews through the studio; sending it live stays a separate
    // operator action.
    await studio.approveDetection(id);
    await Promise.all([refreshQueue(), refreshHistory()]);
  }, [studio, refreshQueue, refreshHistory]);

  const dismiss = useCallback(async (id: string) => {
    setCandidates((await App.DismissDetection(id)) ?? []);
    await refreshHistory();
  }, [refreshHistory]);

  const clear = useCallback(async () => {
    setCandidates((await App.ClearDetections()) ?? []);
    setTranscript('');
    await refreshHistory();
  }, [refreshHistory]);

  // Release the microphone when the studio window goes away.
  useEffect(() => () => stopCapture(), [stopCapture]);

  const value = useMemo<DetectionValue>(() => ({
    listening,
    devices,
    deviceId,
    level,
    transcript,
    candidates,
    history,
    analyzing,
    disabled,
    engineConfigured,
    bufferedSeconds,
    segment,
    repeats,
    autoProjection,
    autoProjectionPaused,
    autoProjected,
    continuous,
    segmenting,
    lastError,
    toggleContinuous,
    setDevice: setDeviceId,
    setAutoProjection: setAutoProjectionEnabled,
    toggleAutoProjectionPause,
    toggleListening,
    analyze,
    analyzeFile,
    approve,
    dismiss,
    clear,
    refresh: async () => {
      await Promise.all([refreshConfig(), refreshQueue()]);
    },
  }), [
    listening, devices, deviceId, level, transcript, candidates, history, analyzing, disabled,
    engineConfigured, bufferedSeconds, segment, repeats, autoProjection, autoProjectionPaused, autoProjected,
    continuous, segmenting, lastError, toggleContinuous, analyzeSegment,
    setAutoProjectionEnabled, toggleAutoProjectionPause, toggleListening, analyze, analyzeFile, approve, dismiss, clear,
    refreshConfig, refreshQueue, refreshHistory,
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
