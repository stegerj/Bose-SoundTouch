import { h } from 'preact';
import { useState } from 'preact/hooks';
import htm from 'htm';
import { api } from '../api.js';

const html = htm.bind(h);

export function DeezerBrowser({ devices, deviceId }) {
    const [sections, setSections] = useState([]);
    const [searchQuery, setSearchQuery] = useState('');
    const [searchType, setSearchType] = useState('album');
    const [loading, setLoading] = useState(false);
    const [statusMessage, setStatusMessage] = useState('');
    const [pendingAction, setPendingAction] = useState(null);

    const targetDeviceId = deviceId || null;
    const deviceEntries = Object.entries(devices || {}).filter(([id, dev]) => id && dev);

    async function explodeItem(item) {
        const actualType = item.type || searchType;
        setLoading(true);
        setStatusMessage(`Caricamento dettagli per ${item.name || item.title}...`);
        try {
            if (actualType === 'album') {
                const res = await api.deezerAlbumTracks(item.id);
                if (res && res.data) {
                    const tracks = res.data.map(t => ({
                        id: t.id,
                        title: t.title || t.name || 'Traccia', // Forza la lettura da t.title dell'API Deezer
                        name: t.title || t.name || 'Traccia',  // Duplica su name per uniformità grafica
                        artist: item.subtitle || item.name || 'Artista', // Eredita l'artista dall'album genitore
                        subtitle: item.subtitle || 'Artista',
                        album: item.name || item.title || '',
                        cover_url: item.imageUrl || item.cover_url || '',
                        type: 'track'
                    }));
                    setSections([{ name: `Tracce dell'Album: ${item.name || item.title}`, items: tracks }]);
                    setStatusMessage('');
                }
} else if (actualType === 'artist') {
                // Esegue la chiamata all'handler Go registrato
                const response = await api.deezerArtistTopTracks(item.id);

                // Estrazione sicura: naviga dentro l'oggetto standard APIResponse di AfterTouch (.data)
                const innerData = response?.data || {};

                // Assicura che gli elementi estratti siano array validi, altrimenti fa fallback su array vuoto []
                const tracksList = Array.isArray(innerData.tracks) ? innerData.tracks : [];
                const albumsList = Array.isArray(innerData.albums) ? innerData.albums : [];

                const formattedAlbums = albumsList.map(a => ({
                    id: a.id,
                    name: a.title,
                    subtitle: item.name,
                    imageUrl: a.cover_medium,
                    type: 'album'
                }));

                const formattedTopTracks = tracksList.map(t => ({
                    id: t.id,
                    title: t.title,
                    name: t.title,
                    artist: item.name,
                    album: t.album?.title || '',
                    cover_url: t.album?.cover_medium || '',
                    type: 'track'
                }));

                setSections([
                    { name: `Brani più popolari di ${item.name} (Top 5)`, items: formattedTopTracks },
                    { name: `Album di ${item.name}`, items: formattedAlbums }
                ]);
                setStatusMessage('');
            }
        } catch (err) {
            console.error(err);
            setStatusMessage('Impossibile esplodere i dettagli.');
        } finally {
            setLoading(false);
        }
    }

    async function handleAction(actionType, item, currentSectionItems = []) {
        if (!item) return;

        const actualType = item.type || searchType;
        const idClean = item.id;
        const finalName = item.name || item.title || 'Sconosciuto';

        let finalDevId = targetDeviceId;
        if (!finalDevId && deviceEntries.length > 0) {
            finalDevId = deviceEntries[0][0];
        }

        // --- PLAY DIRETTO ALBUM O TRACCIA DA RICERCA (VECCHIO STILE NATIVO BOSE) ---
        if (actionType === 'play_context' && (actualType === 'album' || searchType === 'track')) {
            setStatusMessage(`Inizializzazione riproduzione nativa Bose: ${finalName}...`);
            try {
                await api.deezerPlay(finalDevId, {
                    location: String(idClean),
                    type: actualType,
                    itemName: finalName,
                });
                setStatusMessage('');
            } catch (err) {
                console.error('Riproduzione fallita:', err);
                setStatusMessage('Azione fallita. Controlla i log del server.');
            }
            return;
        }

        // --- TRACCE DA CONTESTO ESPLOSO (CON CODA) O ADD TO QUEUE (+) ---
        let payload = null;

              if (actualType === 'track') {
                  payload = {
                      id: Number(idClean),
                      title: item.title || item.name || 'Traccia sconosciuta',
                      artist: item.artist || 'Artista sconosciuto',
                      stream_url: String(idClean), // Richiesto da bmxpkg.QueueTrack
                      cover_url: item.cover_url || item.imageUrl || '' // Richiesto da bmxpkg.QueueTrack
                  };
        } else if (actualType === 'album' && actionType === 'add_queue') {
            setLoading(true);
            try {
                const res = await api.deezerAlbumTracks(idClean);
                if (res?.data) {
                    payload = res.data.map(t => ({
                        id: Number(t.id),
                        title: t.title || t.name || 'Traccia sconosciuta',
                        artist: item.subtitle || 'Artista sconosciuto',
                        album: item.name || item.title,
                        cover_url: item.imageUrl || item.cover_url
                    }));
                }
            } catch (e) {
                console.error(e);
                setLoading(false);
                return;
            } finally {
                setLoading(false);
            }
        }

        const safeItems = Array.isArray(currentSectionItems) ? currentSectionItems : [];
        const contextTracks = safeItems
            .filter(i => i && (i.type === 'track' || searchType === 'track'))
            .map(t => ({
                id: Number(t.id),
                title: t.title || t.name || 'Traccia',
                artist: t.artist || 'Artista',
                stream_url: String(t.id), // Allineato a bmxpkg.QueueTrack
                cover_url: t.cover_url || t.imageUrl || '' // Allineato a bmxpkg.QueueTrack
            }));

        const task = {
            type: actionType,
            item: { ...item, type: actualType },
            payload: payload,
            contextTracks: contextTracks
        };

        if (finalDevId) {
            await executeAction(finalDevId, task);
        } else {
            setPendingAction(task);
        }
    }

    async function executeAction(deviceId, task) {
        setLoading(true);
        try {
            if (task.type === 'play_context') {
                await api.deezerPlayFromContext(deviceId, task.payload.id, task.contextTracks, 'custom_list', String(task.payload.id));
            } else if (task.type === 'add_queue') {
                await api.deezerAddToQueue(deviceId, task.payload);
                setStatusMessage('Elemento aggiunto alla coda visibile.');
                setTimeout(() => setStatusMessage(''), 2000);
            }
        } catch (err) {
            console.error(err);
            setStatusMessage('Azione fallita.');
        } finally {
            setLoading(false);
            setPendingAction(null);
        }
    }
    async function search(q, type) {
        if (!q?.trim()) return;
        setLoading(true);
        setStatusMessage('Ricerca su Deezer...');
        setSections([]);

        try {
            const result = await api.deezerSearch(q, type);
            const itemsList = result?.data;

            if (!Array.isArray(itemsList) || itemsList.length === 0) {
                setStatusMessage('Nessun risultato trovato.');
                setLoading(false);
                return;
            }

            setStatusMessage(`Trovati ${itemsList.length} risultati:`);
            mapAndSetItems(itemsList, type, `${type.charAt(0).toUpperCase() + type.slice(1)} Risultati`);
        } catch (err) {
            console.error(err);
            setStatusMessage('Errore durante il recupero dei dati.');
        } finally {
            setLoading(false);
        }
    }

    function mapAndSetItems(itemsList, defaultType, sectionName) {
        if (!Array.isArray(itemsList)) return;

        const items = itemsList
            .filter(item => {
                if (!item) return false;
                if ((item.type || defaultType) === 'artist' && item.nb_album === 0) return false;
                return true;
            })
            .map(item => {
                const actualType = item.type || defaultType;
                let imageUrl = '';
                let name = '';
                let subtitle = '';

                if (actualType === 'album') {
                    imageUrl = item.cover_medium || item.cover_small || '';
                    name = item.title || '';
                    subtitle = item.artist?.name || '';
                } else if (actualType === 'artist') {
                    imageUrl = item.picture_medium || item.picture_small || '';
                    name = item.name || '';
                    subtitle = item.nb_album !== undefined ? `${item.nb_album} Album` : 'Artista';
                } else if (actualType === 'track') {
                    imageUrl = item.album?.cover_medium || item.album?.cover_small || '';
                    name = item.title || '';
                    subtitle = item.artist?.name || '';
                }

                return { id: item.id, name, subtitle, imageUrl, type: actualType };
            });

        setSections([{ name: sectionName, items }]);
    }

    return html`
        <div class="tunein-browser deezer-browser" style=${{ padding: '16px' }}>
            <div class="tunein-toolbar" style=${{ display: 'flex', gap: '8px', marginBottom: '16px' }}>
                <select
                    class="btn-secondary"
                    style=${{ padding: '0 8px', height: '36px', borderRadius: '4px', background: '#333', color: '#fff', border: 'none' }}
                    value=${searchType}
                    onChange=${(e) => setSearchType(e.target.value)}
                >
                    <option value="album">Albums</option>
                    <option value="artist">Artists</option>
                    <option value="track">Tracks</option>
                </select>

                <input
                    type="text"
                    class="tunein-search-input"
                    style=${{ flex: '1', padding: '0 12px', height: '36px', borderRadius: '4px', background: '#222', color: '#fff', border: '1px solid #444' }}
                    placeholder="Cerca su Deezer..."
                    value=${searchQuery}
                    onInput=${(e) => setSearchQuery(e.target.value)}
                    onKeyDown=${(e) => e.key === 'Enter' && search(searchQuery, searchType)}
                />
                <button class="btn-primary" style=${{ height: '36px', padding: '0 16px', borderRadius: '4px', background: '#007aff', color: '#fff', border: 'none', cursor: 'pointer' }} onClick=${() => search(searchQuery, searchType)}>Cerca</button>
                <button class="btn-secondary" style=${{ height: '36px', padding: '0 16px', borderRadius: '4px', background: '#444', color: '#fff', border: 'none', cursor: 'pointer' }} onClick=${() => {
                    setSearchQuery('');
                    setStatusMessage('');
                    setSections([]);
                }}>Svuota</button>
            </div>

            ${statusMessage ? html`<div class="tunein-item-desc" style=${{ padding: '8px 0', color: '#aaa', fontSize: '14px' }}>${statusMessage}</div>` : null}
            ${loading ? html`<div class="loading-bar" style=${{ height: '3px', background: '#007aff', width: '100%', marginBottom: '16px' }}></div>` : null}

            <!-- LISTA DEI RISULTATI DELLA RICERCA / DETTAGLI ESPLOSI -->
            <div class="browser-sections" style=${{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
                ${sections.map(section => html`
                    <div class="browser-section" key=${section.name}>
                        <h3 style=${{ color: '#fff', borderBottom: '1px solid #333', paddingBottom: '8px', margin: '0 0 12px 0' }}>${section.name}</h3>
                        <div class="browser-items" style=${{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                            ${(section.items || []).map(item => {
                                if (!item) return null;
                                const currentItemType = item.type || searchType;

                                // CORREZIONE CRITICA: Aggiunto il return esplicito richiesto da htm/Preact
                                return html`
                                <div class="browser-item" key=${item.id} style=${{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '10px', background: '#1e1e1e', borderRadius: '6px' }}>
                                    <div style=${{ display: 'flex', alignItems: 'center', gap: '12px', flex: '1', cursor: currentItemType !== 'track' ? 'pointer' : 'default' }} onClick=${() => currentItemType !== 'track' && explodeItem(item)}>
                                        ${item.imageUrl ? html`<img src=${item.imageUrl} style=${{ width: '48px', height: '48px', borderRadius: '4px', objectFit: 'cover' }} />` : null}
                                        <div style=${{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                                            <!-- Forza il titolo in Bianco Brillante con !important logico -->
                                            <span style=${{ color: '#ffffff', fontWeight: '500', fontSize: '15px', display: 'block', lineHeight: '1.2' }}>
                                                ${item.name || item.title || 'Traccia sconosciuta'}
                                            </span>
                                            <!-- Forza l'artista in Grigio Chiaro Leggibile -->
                                            <span style=${{ color: '#aaaaaa', fontSize: '13px', display: 'block', lineHeight: '1.2' }}>
                                                ${item.subtitle || item.artist || ''}
                                            </span>
                                        </div>

                                    </div>
                                    <div style=${{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                                        ${currentItemType !== 'track' ? html`<button class="btn-secondary" style=${{ padding: '6px 10px', fontSize: '13px', background: '#333', color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer' }} onClick=${() => explodeItem(item)}>Esplodi</button>` : null}
                                        <button class="btn-primary" style=${{ padding: '6px 12px', background: '#34c759', color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer' }} onClick=${(e) => { e.stopPropagation(); handleAction('play_context', item, section.items); }}>▶</button>
                                        ${(currentItemType === 'track' || currentItemType === 'album') ? html`<button class="btn-secondary" style=${{ padding: '6px 12px', background: '#007aff', color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer', fontWeight: 'bold' }} onClick=${(e) => { e.stopPropagation(); handleAction('add_queue', item, section.items); }}>+</button>` : null}
                                    </div>
                                </div>
                                `;
                            })}
                        </div>
                    </div>
                `)}
            </div>
        </div>
    `;
}
