import { useEffect, useState } from 'react';
import { Window } from '@wailsio/runtime';
import { App } from '../../../bindings/zopresent';
import { Icon } from '../../components/Icon';

type Draft = {
  id: string;
  label: string;
  text: string;
};

const emptyDraft = (): Draft => ({ id: '', label: '', text: '' });

function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  return String(error);
}

export function SongEditorWindow({ songID }: { songID: string }) {
  const editing = songID !== '';
  const [title, setTitle] = useState('');
  const [author, setAuthor] = useState('');
  const [copyright, setCopyright] = useState('');
  const [ccliNumber, setCcliNumber] = useState('');
  const [source, setSource] = useState('');
  const [drafts, setDrafts] = useState<Draft[]>([emptyDraft()]);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (!editing) return;

    App.ListSongs('')
      .then((library) => {
        const song = (library?.songs ?? []).find((item) => item.id === songID) ?? null;
        if (!song) {
          setProblem('This song is no longer in the library.');
          return;
        }
        setTitle(song.title);
        setAuthor(song.author);
        setCopyright(song.copyright);
        setCcliNumber(song.ccliNumber ?? '');
        setSource(song.source ?? '');
        setDrafts((song.sections ?? []).map((section) => ({
          id: section.id, label: section.label, text: section.text,
        })));
      })
      .catch((error: unknown) => setProblem(errorMessage(error)));
  }, [editing, songID]);

  function patch(index: number, next: Partial<Draft>) {
    setSaved(false);
    setDrafts((current) => current.map((draft, position) => (
      position === index ? { ...draft, ...next } : draft
    )));
  }

  function remove(index: number) {
    setSaved(false);
    setDrafts((current) => current.filter((_, position) => position !== index));
  }

  async function save() {
    setBusy(true);
    setProblem(null);
    try {
      const sections = drafts.map((draft) => ({
        label: draft.label, text: draft.text, id: draft.id,
      }));
      if (editing) {
        await App.UpdateSong({ id: songID, title, author, copyright, ccliNumber, source, sections });
      } else {
        await App.CreateSong({
          title, author, copyright, ccliNumber, source,
          sections: sections.map(({ label, text }) => ({ label, text })),
        });
      }
      setSaved(true);
      // The studio rereads the library when its tab is next opened, so the
      // editor can get out of the way once the work is stored.
      Window.Close();
    } catch (error: unknown) {
      setProblem(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }

  const canSave = title.trim() !== ''
    && drafts.length > 0
    && drafts.every((draft) => draft.label.trim() !== '' && draft.text.trim() !== '');

  return (
    <main className="grid grid-rows-[auto_auto_minmax(0,1fr)_auto_auto] gap-3.5 h-full pt-3.5 px-4 pb-4 overflow-hidden" aria-label={editing ? `Edit ${title || 'song'}` : 'New song'}>
      <header className="flex items-start justify-between gap-3 [&_h2]:truncate [&_h2]:text-[17px] [&_h2]:font-semibold [&_h2]:mt-[3px]">
        <div>
          <p className="text-[9.5px] uppercase tracking-[1.3px] font-semibold text-secondary">{editing ? 'Edit song' : 'New song'}</p>
          <h2>{title.trim() || 'Untitled song'}</h2>
        </div>
        <button type="button" className="inline-grid place-items-center w-[34px] h-[34px] border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50" onClick={() => Window.Close()} aria-label="Close editor" title="Close">
          <Icon name="close" size={16} />
        </button>
      </header>

      <div className="grid grid-cols-[2fr_1fr_1fr] gap-2.5 [&>label]:grid [&>label]:gap-1 [&>label]:min-w-0 [&_span]:text-secondary [&_span]:text-[9.5px] [&_span]:tracking-[1px] [&_span]:uppercase [&_input]:min-w-0 [&_input]:h-8 [&_input]:px-2.5 [&_input]:border [&_input]:border-[#35445C] [&_input]:rounded-md [&_input]:bg-app [&_input]:text-text [&_input]:text-[12px] [&_input]:focus:border-blue [&_input]:focus:outline-none [&_input]:placeholder:text-[#6F809B]">
        <label>
          <span>Title</span>
          <input value={title} onChange={(event) => { setTitle(event.target.value); setSaved(false); }} autoComplete="off" />
        </label>
        <label>
          <span>Author</span>
          <input value={author} onChange={(event) => { setAuthor(event.target.value); setSaved(false); }} autoComplete="off" />
        </label>
        <label>
          <span>Copyright</span>
          <input value={copyright} onChange={(event) => { setCopyright(event.target.value); setSaved(false); }} autoComplete="off" placeholder="Public domain" />
        </label>
        <label>
          <span>CCLI #</span>
          <input value={ccliNumber} onChange={(event) => { setCcliNumber(event.target.value); setSaved(false); }} autoComplete="off" placeholder="1234567" />
        </label>
        <label>
          <span>Source</span>
          <input value={source} onChange={(event) => { setSource(event.target.value); setSaved(false); }} autoComplete="off" placeholder="Songbook / album" />
        </label>
      </div>

      <section className="min-h-0 overflow-y-auto grid gap-2.5 content-start pr-1" aria-label="Sections">
        {drafts.map((draft, index) => (
          <div
            className="relative grid gap-1.5 p-2.5 border border-border rounded-lg bg-[#0C1626] [&_input]:min-w-0 [&_input]:h-[30px] [&_input]:pl-2.5 [&_input]:pr-10 [&_input]:border [&_input]:border-[#35445C] [&_input]:rounded-md [&_input]:bg-app [&_input]:text-text [&_input]:text-[12px] [&_input]:focus:border-blue [&_input]:focus:outline-none [&_input]:placeholder:text-[#6F809B] [&_textarea]:min-w-0 [&_textarea]:min-h-[92px] [&_textarea]:px-2.5 [&_textarea]:py-2 [&_textarea]:border [&_textarea]:border-[#35445C] [&_textarea]:rounded-md [&_textarea]:bg-app [&_textarea]:text-text [&_textarea]:text-[12px] [&_textarea]:[font-family:inherit] [&_textarea]:resize-y [&_textarea]:leading-[1.6] [&_textarea]:focus:border-blue [&_textarea]:focus:outline-none [&_textarea]:placeholder:text-[#6F809B]"
            key={`${draft.id}-${index}`}
          >
            <input
              value={draft.label}
              placeholder="Verse 1"
              aria-label={`Section ${index + 1} label`}
              onChange={(event) => patch(index, { label: event.target.value })}
            />
            <textarea
              value={draft.text}
              placeholder={'Write the lyrics, one line per slide line'}
              aria-label={`Section ${index + 1} lyrics`}
              onChange={(event) => patch(index, { text: event.target.value })}
            />
            <button
              type="button"
              className="inline-grid place-items-center w-7 h-7 border border-transparent rounded-md bg-transparent text-secondary hover:not-disabled:bg-elevated hover:not-disabled:text-text disabled:opacity-50 absolute right-2 top-[9px]"
              disabled={drafts.length === 1}
              aria-label={`Remove section ${index + 1}`}
              title="Remove section"
              onClick={() => remove(index)}
            >
              <Icon name="close" size={13} />
            </button>
          </div>
        ))}
        <button type="button" className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-[#35445C] rounded-md bg-elevated text-[10.5px] font-medium whitespace-nowrap" onClick={() => setDrafts((current) => [...current, emptyDraft()])}>
          <Icon name="plus" size={13} />Add section
        </button>
      </section>

      {problem && <p className="text-error text-[11.5px]" role="alert">{problem}</p>}
      {saved && !problem && <p className="text-teal text-[11.5px]">Saved.</p>}

      <footer className="flex items-center justify-between gap-3.5">
        <span className="text-[#6F809B] text-[10.5px]">
          Sections keep their identity, so anything already scheduled stays put.
        </span>
        <button
          type="button"
          className="min-h-7 inline-flex items-center justify-center gap-1.5 px-[9px] py-1 border border-blue! rounded-md bg-blue! text-white! text-[10.5px] font-medium whitespace-nowrap disabled:text-secondary! disabled:bg-[#142033]! disabled:border-[#2A3950]! not-disabled:hover:bg-[#5B88FF]! not-disabled:hover:border-[#5B88FF]!"
          disabled={busy || !canSave}
          onClick={() => void save()}
        >
          {editing ? 'Save changes' : 'Add song'}
        </button>
      </footer>
    </main>
  );
}
