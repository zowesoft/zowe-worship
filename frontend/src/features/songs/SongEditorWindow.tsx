import { useEffect, useState } from 'react';
import { Window } from '@wailsio/runtime';
import { App } from '../../../bindings/zowe-worship';
import { Icon } from '../../components/Icon';
import './song-editor.css';

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
        await App.UpdateSong({ id: songID, title, author, copyright, sections });
      } else {
        await App.CreateSong({
          title, author, copyright,
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
    <main className="song-editor" aria-label={editing ? `Edit ${title || 'song'}` : 'New song'}>
      <header className="song-editor-header">
        <div>
          <p className="eyebrow">{editing ? 'Edit song' : 'New song'}</p>
          <h2>{title.trim() || 'Untitled song'}</h2>
        </div>
        <button type="button" className="icon-button" onClick={() => Window.Close()} aria-label="Close editor" title="Close">
          <Icon name="close" size={16} />
        </button>
      </header>

      <div className="song-editor-fields">
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
      </div>

      <section className="song-editor-sections" aria-label="Sections">
        {drafts.map((draft, index) => (
          <div className="song-editor-section" key={`${draft.id}-${index}`}>
            <input
              className="song-editor-label"
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
              className="icon-button icon-button--small song-editor-remove"
              disabled={drafts.length === 1}
              aria-label={`Remove section ${index + 1}`}
              title="Remove section"
              onClick={() => remove(index)}
            >
              <Icon name="close" size={13} />
            </button>
          </div>
        ))}
        <button type="button" className="button button--small" onClick={() => setDrafts((current) => [...current, emptyDraft()])}>
          <Icon name="plus" size={13} />Add section
        </button>
      </section>

      {problem && <p className="song-editor-error" role="alert">{problem}</p>}
      {saved && !problem && <p className="song-editor-saved">Saved.</p>}

      <footer className="song-editor-footer">
        <span className="song-editor-hint">
          Sections keep their identity, so anything already scheduled stays put.
        </span>
        <button
          type="button"
          className="button button--primary button--small"
          disabled={busy || !canSave}
          onClick={() => void save()}
        >
          {editing ? 'Save changes' : 'Add song'}
        </button>
      </footer>
    </main>
  );
}
