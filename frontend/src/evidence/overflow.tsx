import { StrictMode, useCallback, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { ThemedSlide } from '../components/ThemedSlide';
import type { Theme } from '../../bindings/zowe-worship/internal/themes/models';
import { BackgroundKind, Kind, TextCase } from '../../bindings/zowe-worship/internal/themes/models';
import '../styles/tokens.css';
import '../styles/global.css';

// Overflow acceptance evidence: the same themed slide drawn on a stage sized
// like every output shape the app has to fill - 16:9 and 4:3 projectors, an
// ultrawide bar, a portrait screen - with a passage and reference long enough
// to overflow if the fit did not work. Each stage reports its own verdict
// after ThemedSlide's fit pass, so a screenshot of this page is the capture:
// "fits" means the words and reference shrank together until they sat inside
// the stage; only a hit on the 30% floor while still overflowing would read
// as overflowed.
//
// Regenerate the capture with `npm run dev`, open
// http://127.0.0.1:9245/evidence-overflow.html and screenshot it.

const demanding: Theme = {
  id: 'evidence',
  name: 'Overflow probe',
  kind: Kind.KindScripture,
  isDefault: false,
  style: {
    fontFamily: 'Georgia, "Times New Roman", serif',
    fontSize: 5.6,
    lineHeight: 1.6,
    bold: false,
    italic: false,
    align: 'center',
    color: '#F3F6FC',
    case: TextCase.CaseAsWritten,
    shadow: true,
    margin: 10,
  },
  background: { kind: BackgroundKind.BackgroundSolid, color: '#081A2E', color2: '#081A2E', angle: 180, mediaId: '' },
  reference: {
    placement: 'bottom',
    fontFamily: '',
    fontSize: 2.6,
    color: '#9FB4D4',
    align: 'center',
    case: TextCase.CaseUppercase,
    bold: false,
    italic: false,
  },
  createdAt: '',
  updatedAt: '',
};

const passage = [
  'Thy word is a lamp unto my feet',
  'and a light unto my path.',
  'I have sworn, and I will perform it,',
  'that I will keep thy righteous judgments.',
  'I am afflicted very much:',
  'quicken me, O LORD, according unto thy word.',
  'Accept, I beseech thee, the free-will offerings of my mouth,',
  'O LORD, and teach me thy judgments.',
  'My soul is continually in my hand:',
  'yet do I not forget thy law.',
  'The wicked have laid a snare for me:',
  'yet I erred not from thy precepts.',
];

const reference = 'Psalms 119:105-116 · King James Version · Public Domain';

const stages = [
  { label: '1920×1080 · 16:9 projector', width: 640, height: 360 },
  { label: '1440×1080 · 4:3 projector', width: 480, height: 360 },
  { label: '2560×1080 · 21:9 ultrawide', width: 640, height: 270 },
  { label: '1080×1920 · 9:16 portrait', width: 270, height: 480 },
] as const;

function Probe({ label, width, height }: { label: string; width: number; height: number }) {
  const linesRef = useRef<HTMLParagraphElement>(null);
  const [verdict, setVerdict] = useState('measuring…');

  // Called by ThemedSlide after every fit pass; the comparison is the same
  // one the component uses, read back from the DOM it just wrote.
  const report = useCallback((scale: number) => {
    const element = linesRef.current;
    const box = element?.parentElement;
    if (!element || !box) return;
    const fits = element.scrollHeight <= box.clientHeight + 1;
    if (!fits) {
      setVerdict(`overflowed at the ${(scale * 100).toFixed(0)}% floor`);
    } else if (scale >= 1) {
      setVerdict('fits at full size');
    } else {
      setVerdict(`fits · shrank to ${(scale * 100).toFixed(0)}%`);
    }
  }, []);

  const passed = verdict.startsWith('fits');

  return (
    <figure className="m-0 grid gap-2">
      <figcaption className="flex items-center justify-between gap-3 text-[12px] text-[#9FB4D4]">
        <span>{label}</span>
        <span
          className={`px-2 py-0.5 rounded-md text-[11px] font-semibold ${
            passed ? 'bg-[#0E2C22] text-[#4ADE80]' : 'bg-[#2C1418] text-[#F87171]'
          }`}
        >
          {verdict}
        </span>
      </figcaption>
      <div className="border border-[#30405C] rounded-md overflow-hidden" style={{ width, height }}>
        <ThemedSlide
          theme={demanding}
          lines={passage}
          reference={reference}
          linesRef={linesRef}
          onFit={report}
          className="w-full h-full"
        />
      </div>
    </figure>
  );
}

function OverflowEvidence() {
  return (
    <main className="min-h-screen bg-[#080F1C] text-[#F3F6FC] p-6 grid gap-5 content-start">
      <header className="grid gap-1">
        <h1 className="m-0 text-[15px] font-semibold">Theme overflow across output aspect ratios</h1>
        <p className="m-0 text-[12px] leading-[1.7] text-[#9FB4D4] max-w-[900px]">
          One demanding theme (large size, wide margins, long reference) on stages shaped like the outputs the app
          drives. The verdict is measured after each fit pass: the passage and reference sit inside the stage when
          the words shrank to fit, and the capture shows the shrunken type, not a clipped line.
        </p>
      </header>
      <div className="flex flex-wrap items-start gap-6">
        {stages.map((stage) => (
          <Probe key={stage.label} label={stage.label} width={stage.width} height={stage.height} />
        ))}
      </div>
    </main>
  );
}

const root = document.getElementById('root');
if (!root) throw new Error('The evidence root element is missing.');

createRoot(root).render(
  <StrictMode>
    <OverflowEvidence />
  </StrictMode>,
);
