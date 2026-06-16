import { h } from 'preact';
import htm from 'htm';
import { api } from '../api.js';

const html = htm.bind(h);

export function DeezerQueueView({ deviceId, queueState, onQueueUpdated }) {
    const { local_queue } = queueState;

    // Rimuove un brano singolo dalla coda tramite la sua posizione (indice)
    const handleRemove = async (index) => {
        try {
            await api.deezerRemoveFromQueue(deviceId, index);
            const updated = await api.deezerGetQueue(deviceId);
            onQueueUpdated(updated);
        } catch (e) {
            console.error("Errore durante la rimozione del brano:", e);
        }
    };

    // Gestisce lo stop e svuota l'intera coda locale
    const handleStopQueue = async () => {
        try {
            await api.deezerQueueStop(deviceId);
            onQueueUpdated({ local_queue: [], current_track: null });
        } catch (e) {
            console.error("Errore durante l'interruzione della coda:", e);
        }
    };

    // Se non ci sono brani inseriti a mano, mostra un messaggio pulito
    if (!local_queue || local_queue.length === 0) {
        return html`
            <div class="deezer-queue-section empty" style="margin: 20px 0; padding: 15px; background: #1e1e1e; border-radius: 8px;">
                <h3 style="margin-top:0; color: #fff;">Prossimi brani in coda</h3>
                <p style="color: #888; font-size: 14px;">La coda è vuota. Usa "Aggiungi alla coda" mentre esplori tracce o album.</p>
            </div>
        `;
    }

    return html`
        <div class="deezer-queue-section" style="margin: 20px 0; padding: 15px; background: #1e1e1e; border-radius: 8px;">
            <div class="queue-header" style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px;">
                <h3 style="margin:0; color: #fff;">Prossimi brani in coda (${local_queue.length})</h3>
                <button class="btn-stop-queue" onClick=${handleStopQueue} style="background: #ff3b30; color: white; border: none; padding: 6px 12px; border-radius: 4px; cursor: pointer; font-weight: bold;">
                    Ferma e Svuota
                </button>
            </div>
            <div class="queue-list" style="display: flex; flex-direction: column; gap: 8px; max-height: 300px; overflow-y: auto;">
                ${local_queue.map((track, index) => html`
                    <div class="queue-item" key=${track.id}-${index} style="display: flex; align-items: center; background: #2a2a2a; padding: 8px; border-radius: 6px; justify-content: space-between;">
                        <div style="display: flex; align-items: center; gap: 10px;">
                            <img src=${track.cover_url || '/app/static/img/logo.svg'} class="queue-cover" style="width: 40px; height: 40px; border-radius: 4px; object-fit: cover;" />
                            <div class="queue-meta" style="display: flex; flex-direction: column;">
                                <span class="queue-title" style="color: #fff; font-weight: 500; font-size: 14px;">${track.title}</span>
                                <span class="queue-artist" style="color: #aaa; font-size: 12px;">${track.artist}</span>
                            </div>
                        </div>
                        <button class="btn-remove-queue" onClick=${() => handleRemove(index)} title="Rimuovi traccia" style="background: transparent; color: #888; border: none; font-size: 16px; cursor: pointer; padding: 5px 10px;">
                            ✕
                        </button>
                    </div>
                `)}
            </div>
        </div>
    `;
}
