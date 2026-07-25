import { h } from 'preact';
import htm from 'htm';
import { api } from '../api.js';

const html = htm.bind(h);

// Shared styles for consistency and performance
const S = {
  container: { margin: '20px 0', padding: '15px', background: '#1e1e1e', borderRadius: '8px' },
  section: { marginBottom: '20px' },
  header: { display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' },
  title: { margin: 0, color: '#fff', fontWeight: 600, fontSize: '14px' },
  subtitle: { margin: 0, color: '#888', fontSize: '12px', fontWeight: 500 },
  stopBtn: { background: '#ff3b30', color: '#fff', border: 'none', padding: '6px 12px', borderRadius: '4px', cursor: 'pointer', fontWeight: 'bold' },
  list: { display: 'flex', flexDirection: 'column', gap: '8px', maxHeight: '250px', overflowY: 'auto' },
  item: { display: 'flex', alignItems: 'center', background: '#2a2a2a', padding: '8px', borderRadius: '6px', justifyContent: 'space-between' },
  itemPlaying: { display: 'flex', alignItems: 'center', background: '#3a3a3a', padding: '10px', borderRadius: '6px', justifyContent: 'space-between', border: '1px solid #4a4a4a' },
  itemLeft: { display: 'flex', alignItems: 'center', gap: '10px' },
  cover: { width: '48px', height: '48px', borderRadius: '4px', objectFit: 'cover', flexShrink: 0 },
  coverSmall: { width: '40px', height: '40px', borderRadius: '4px', objectFit: 'cover', flexShrink: 0 },
  meta: { display: 'flex', flexDirection: 'column' },
  trackTitle: { color: '#fff', fontWeight: 500, fontSize: '14px' },
  trackTitlePlaying: { color: '#fff', fontWeight: 600, fontSize: '15px' },
  trackArtist: { color: '#aaa', fontSize: '12px' },
  removeBtn: { background: 'transparent', color: '#888', border: 'none', fontSize: '16px', cursor: 'pointer', padding: '5px 10px' },
  emptyText: { color: '#888', fontSize: '14px' },
  playingIndicator: { color: '#4cd964', fontSize: '12px', fontWeight: 600, marginRight: '8px' },
};

export function DeezerQueueView({ deviceId, queueState, onQueueUpdated }) {
    const { local_queue, current_track } = queueState;

    // Remove a single track from the queue by its index
    const handleRemove = async (index) => {
        try {
            await api.deezerQueueRemove(deviceId, index);
            const updated = await api.deezerQueueStatus(deviceId);
            onQueueUpdated(updated);
        } catch (e) {
            console.error('[queue remove]', e);
        }
    };

    // Stop playback and clear the entire queue
    const handleStopQueue = async () => {
        try {
            await api.deezerQueueStop(deviceId);
            onQueueUpdated({ local_queue: [], current_track: null });
        } catch (e) {
            console.error('[queue stop]', e);
        }
    };

    const hasContent = (current_track) || (local_queue && local_queue.length > 0);

    // Show empty state message when no tracks are queued
    if (!hasContent) {
        return html`
            <div class="deezer-queue-section empty" style=${S.container}>
                <h3 style=${S.title}>Coda Deezer</h3>
                <p style=${S.emptyText}>La coda è vuota. Usa "Aggiungi alla coda" mentre esplori tracce o album.</p>
            </div>
        `;
    }

    return html`
        <div class="deezer-queue-section" style=${S.container}>
            ${current_track ? html`
                <div style=${S.section}>
                    <div class="queue-header" style=${S.header}>
                        <div>
                            <h3 style=${S.title}>In riproduzione</h3>
                            <p style=${S.subtitle}>1 brano</p>
                        </div>
                    </div>
                    <div class="queue-list" style=${{ ...S.list, maxHeight: 'none' }}>
                        <div class="queue-item-playing" style=${S.itemPlaying}>
                            <div style=${S.itemLeft}>
                                <span style=${S.playingIndicator}>▶</span>
                                <img src=${current_track.cover_url || '/app/static/img/logo.svg'} class="queue-cover" style=${S.cover} alt="" />
                                <div class="queue-meta" style=${S.meta}>
                                    <span class="queue-title" style=${S.trackTitlePlaying}>${current_track.title}</span>
                                    <span class="queue-artist" style=${S.trackArtist}>${current_track.artist}</span>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            ` : null}

            ${local_queue && local_queue.length > 0 ? html`
                <div style=${S.section}>
                    <div class="queue-header" style=${S.header}>
                        <div>
                            <h3 style=${S.title}>Prossimi brani</h3>
                            <p style=${S.subtitle}>${local_queue.length} brani in coda</p>
                        </div>
                        <button class="btn-stop-queue" onClick=${handleStopQueue} style=${S.stopBtn}>
                            Ferma e Svuota
                        </button>
                    </div>
                    <div class="queue-list" style=${S.list}>
                        ${local_queue.map((track, index) => html`
                            <div class="queue-item" key=${track.id}-${index} style=${S.item}>
                                <div style=${S.itemLeft}>
                                    <img src=${track.cover_url || '/app/static/img/logo.svg'} class="queue-cover" style=${S.coverSmall} alt="" />
                                    <div class="queue-meta" style=${S.meta}>
                                        <span class="queue-title" style=${S.trackTitle}>${track.title}</span>
                                        <span class="queue-artist" style=${S.trackArtist}>${track.artist}</span>
                                    </div>
                                </div>
                                <button class="btn-remove-queue" onClick=${() => handleRemove(index)} title="Rimuovi traccia" style=${S.removeBtn}>
                                    ✕
                                </button>
                            </div>
                        `)}
                    </div>
                </div>
            ` : null}
        </div>
    `;
}
