import { h } from 'preact';
import { useState, useEffect } from 'preact/hooks';
import htm from 'htm';
import { api } from '../api.js';

const html = htm.bind(h);

const pillBtnStyle = {
  padding: '6px 12px',
  borderRadius: '4px',
  border: 'none',
  color: '#fff',
  cursor: 'pointer',
  fontSize: '13px',
};

function buildTrackItem({ id, title, artistName, albumName, coverUrl, preview }) {
  return {
    id: Number(id),
    title: title || 'Traccia sconosciuta',
    name: title || 'Traccia sconosciuta',
    artistName: artistName || 'Artista sconosciuto',
    subtitle: artistName || 'Artista sconosciuto',
    album: albumName || '',
    imageUrl: coverUrl || '',
    cover_url: coverUrl || '',
    stream_url: preview || '',
    type: 'track',
  };
}

export function DeezerBrowser({ devices, deviceId }) {
  const [sections, setSections] = useState([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [searchType, setSearchType] = useState('album');
  const [loading, setLoading] = useState(false);
  const [statusMessage, setStatusMessage] = useState('');
  const [pendingAction, setPendingAction] = useState(null);
  const [expandedItem, setExpandedItem] = useState(null);
  const [expandedTracks, setExpandedTracks] = useState([]);
  const [queueSnapshot, setQueueSnapshot] = useState({ tracks: [], playing: false, pos: 0 });

  // Log per verificare i dati delle sezioni
  useEffect(() => {
    console.log('Sezioni aggiornate:', sections);
  }, [sections]);

   const handleAction = async (actionType, item, sectionItems = []) => {
     if (!item) return;
     const currentType = item.type || searchType;

     // SCENARIO 1: Gestione dell'esplosione dell'album o dell'artista (Header Fisso)
     if (actionType === 'explode') {
       setLoading(true);
       setStatusMessage(`Caricamento dettagli per "${item.name || item.title}"...`);
       try {
         let tracksData = [];
         if (currentType === 'album') {
           const res = await api.deezerAlbumTracks(item.id);
           tracksData = res.data || res || [];
         } else if (currentType === 'artist') {
           const res = await api.deezerArtistTracklist(item.id);
           tracksData = res.data || res || [];
         }
         setExpandedItem(item);
         setExpandedTracks(Array.isArray(tracksData) ? tracksData : []);
         setStatusMessage('');
       } catch (err) {
         setStatusMessage(`Errore nel caricamento dei dettagli: ${err.message}`);
       } finally {
         setLoading(false);
       }
       return;
     }

     // SCENARIO 2: Gestione dell'artista (Tracklist / Coda)
     if (currentType === 'artist') {
       // Usiamo solo il deviceId dello stato principale. Se è undefined, lascerà fare al popup pending
       await playOrQueueArtistTracklist(actionType, item, deviceId);
       return;
     }

     // SCENARIO 3: Calcolo del payload e della coda nascosta ciclica per Tracce e Album
     let payload = [];
     let contextTracks = [];

     if (currentType === 'track') {
       const trackItemForm = buildTrackItem({
         id: item.id,
         title: item.title || item.name,
         artistName: item.artist || item.subtitle || 'Artista',
         albumName: item.album || '',
         coverUrl: item.cover_url || item.imageUrl || '',
         preview: String(item.id), // ID nativo per far suonare il brano intero
       });
       payload = [trackItemForm];

       // Generazione della coda a partire dal brano cliccato
       const allTrackItems = (Array.isArray(sectionItems) ? sectionItems : [])
         .filter(i => i && (i.type || searchType) === 'track');
       const clickedIndex = allTrackItems.findIndex(t => String(t.id) === String(item.id));

       let orderedItems = allTrackItems;
       if (clickedIndex >= 0) {
         orderedItems = [...allTrackItems.slice(clickedIndex), ...allTrackItems.slice(0, clickedIndex)];
       }

       contextTracks = orderedItems.map(t => buildTrackItem({
         id: t.id,
         title: t.title || t.name,
         artistName: t.artist || t.subtitle || item.artist || 'Artista',
         albumName: t.album || '',
         coverUrl: t.cover_url || t.imageUrl || '',
         preview: String(t.id),
       })).filter(t => t && t.id);

     } else if (currentType === 'album') {
       setLoading(true);
       try {
         const res = await api.deezerAlbumTracks(item.id);
         const albumTracks = res.data || res || [];
         payload = albumTracks.map(t => buildTrackItem({
           id: t.id,
           title: t.title || t.name,
           artistName: item.artist || item.name || 'Artista',
           albumName: item.title || item.name || '',
           coverUrl: item.imageUrl || item.cover_url || '',
           preview: String(t.id),
         })).filter(t => t && t.id);
         contextTracks = payload;
       } catch (err) {
         console.error(err);
         setStatusMessage("Impossibile caricare le tracce dell'album.");
         setLoading(false);
         return;
       } finally {
         setLoading(false);
       }
     }

     const task = {
       type: actionType,
       item: { ...item, type: currentType },
       payload,
       contextTracks
     };

     // Se c'è un dispositivo attivo selezionato a livello globale esegue subito,
     // altrimenti imposta l'azione come pendente per mostrare il selettore delle casse
     if (deviceId) {
       await executeAction(deviceId, task);
     } else {
       setPendingAction(task);
     }
   };

   const playOrQueueArtistTracklist = async (actionType, artistItem, targetDevId) => {
     setLoading(true);
     try {
       const res = await api.deezerArtistTracklist(artistItem.id);
       const tracks = res?.data || res || [];

       const mappedTracks = tracks
         .map(t => buildTrackItem({
           id: t.id,
           title: t.title || t.name,
           artistName: artistItem.name || artistItem.title || 'Artista',
           albumName: t.album?.title || '',
           coverUrl: t.album?.cover_medium || artistItem.imageUrl || '',
           preview: String(t.id), // ID nativo
         }))
         .filter(t => t && t.id);

       if (mappedTracks.length === 0) {
         setStatusMessage('Nessuna traccia riproducibile trovata per questo artista.');
         return;
       }

       const task = {
         type: actionType,
         item: { ...artistItem, type: 'artist' },
         payload: mappedTracks,
         contextTracks: mappedTracks,
       };

       if (targetDevId) {
         await executeAction(targetDevId, task);
       } else {
         setPendingAction(task);
       }
     } catch (err) {
       console.error(err);
       setStatusMessage("Impossibile caricare i brani dell'artista.");
     } finally {
       setLoading(false);
     }
   };

  const executeAction = async (deviceId, action) => {
    const { type, item, payload, contextTracks } = action;
    if (type === 'play_native') {
      try {
        setLoading(true);
        await api.deezerPlay(deviceId, {
          location: item.id.toString(),
          itemName: item.name || item.title,
          type: item.type,
        });
        setStatusMessage(`In riproduzione nativa: ${item.name || item.title}`);
      } catch (err) {
        setStatusMessage(`Errore riproduzione nativa: ${err.message}`);
      } finally {
        setLoading(false);
        setPendingAction(null);
      }
      return;
    }

    if (type === 'play_context') {
      try {
        setLoading(true);
        const tracksToQueue = (contextTracks && contextTracks.length > 0) ? contextTracks : payload;
        console.log('Tracce da mettere in coda (play_context):', tracksToQueue);
        if (!tracksToQueue || tracksToQueue.length === 0) {
          setStatusMessage('Nessuna traccia valida per la riproduzione contestuale.');
          return;
        }
        await api.deezerQueue(deviceId, tracksToQueue);
        setStatusMessage(`Coda contestuale avviata per: ${item.name || item.title}`);
        await refreshQueueStatus(deviceId);
      } catch (err) {
        console.error(err);
        setStatusMessage(`Errore coda contestuale: ${err.message}`);
      } finally {
        setLoading(false);
        setPendingAction(null);
      }
    } else if (type === 'add_queue') {
      try {
        setLoading(true);
        if (!payload || payload.length === 0) {
          setStatusMessage('Nessuna traccia valida da aggiungere in coda.');
          return;
        }
        console.log('Aggiungo tracce in coda:', payload);
        await api.deezerAddToQueue(deviceId, payload);
        setStatusMessage(`Aggiunto in coda: ${item.name || item.title}`);
        await refreshQueueStatus(deviceId);
      } catch (err) {
        console.error('Errore durante aggiunta in coda:', err);
        setStatusMessage(`Errore aggiunta in coda: ${err.message}`);
      } finally {
        setLoading(false);
        setPendingAction(null);
      }
    }
  };

  async function refreshQueueStatus(devId) {
    if (!devId) return;
    try {
      const status = await api.deezerQueueStatus(devId);
      if (status) {
        setQueueSnapshot({
          tracks: status.tracks || [],
          playing: !!status.playing,
          pos: status.pos || 0,
        });
      }
    } catch (err) {
      console.error("Errore aggiornamento coda:", err);
    }
  }

  async function playQueue() {
    const devId = deviceId || (Object.keys(devices || {}).length === 1 ? Object.keys(devices)[0] : null);
    if (!devId) {
      setStatusMessage('Seleziona prima un dispositivo.');
      return;
    }
    setLoading(true);
    try {
      await api.deezerQueuePlay(devId);
      await refreshQueueStatus(devId);
    } catch (err) {
      console.error(err);
      setStatusMessage('Impossibile avviare la coda.');
    } finally {
      setLoading(false);
    }
  }

  async function stopQueue() {
    const devId = deviceId || (Object.keys(devices || {}).length === 1 ? Object.keys(devices)[0] : null);
    if (!devId) return;
    setLoading(true);
    try {
      await api.deezerQueueStop(devId);
      await refreshQueueStatus(devId);
    } catch (err) {
      console.error(err);
      setStatusMessage('Impossibile fermare la coda.');
    } finally {
      setLoading(false);
    }
  }

  async function removeQueueItem(index) {
    const devId = deviceId || (Object.keys(devices || {}).length === 1 ? Object.keys(devices)[0] : null);
    if (!devId) return;
    setLoading(true);
    try {
      await api.deezerQueueRemove(devId, index);
      await refreshQueueStatus(devId);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  }

  async function clearQueue() {
    const devId = deviceId || (Object.keys(devices || {}).length === 1 ? Object.keys(devices)[0] : null);
    if (!devId) return;
    setLoading(true);
    try {
      await api.deezerQueueClear(devId);
      await refreshQueueStatus(devId);
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  }

  async function search(q, type) {
    if (!q?.trim()) return;
    setLoading(true);
    setStatusMessage('Ricerca su Deezer...');
    setSections([]);
    setExpandedItem(null);
    setExpandedTracks([]);

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
        if (actualType === 'track') {
          return buildTrackItem({
            id: item.id,
            title: item.title,
            artistName: item.artist?.name || '',
            albumName: item.album?.title || '',
            coverUrl: item.album?.cover_medium || item.album?.cover_small || '',
            preview: item.preview,
          });
        }
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
        }
        return { id: item.id, name, subtitle, imageUrl, cover_url: imageUrl, type: actualType };
      });
    setSections([{ name: sectionName, items }]);
  }

  async function resolveAlbumTracks(albumItem) {
    if (!albumItem?.id) return [];
    const res = await api.deezerAlbumTracks(albumItem.id);
    const tracks = res?.data || [];
    return tracks
      .map(t => buildTrackItem({
        id: t.id,
        title: t.title,
        artistName: t.artist?.name || 'Artista',
        albumName: albumItem.name || '',
        coverUrl: albumItem.imageUrl || '',
        preview: t.preview,
      }))
      .filter(t => t.stream_url);
  }

  async function expandItemDetails(item) {
    setLoading(true);
    setExpandedTracks([]);
    setExpandedItem(item);
    try {
      if (item.type === 'album') {
        const tracks = await resolveAlbumTracks(item);
        setExpandedTracks(tracks);
      } else if (item.type === 'artist') {
        const res = await api.deezerArtistTopTracks(item.id);
        const tracks = res?.data || [];
        setExpandedTracks(tracks.map(t => buildTrackItem({
          id: t.id,
          title: t.title,
          artistName: item.name || 'Artista',
          albumName: t.album?.title || '',
          coverUrl: t.album?.cover_medium || '',
          preview: t.preview,
        })));
      }
    } catch (err) {
      console.error(err);
      setStatusMessage("Errore nel caricamento dei dettagli.");
    } finally {
      setLoading(false);
    }
  }

  // Stili
  const stylePlayClassico = { ...pillBtnStyle, background: '#007aff' };
  const stylePlayContext = { ...pillBtnStyle, background: '#34c759' };
  const styleAddQueue = { ...pillBtnStyle, background: '#444' };
  const styleCloseBtn = { ...pillBtnStyle, background: 'transparent', border: '1px solid #555' };
  const styleTrackPlay = { padding: '4px 10px', background: '#34c759', color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer' };
  const styleTrackAdd = { padding: '4px 10px', background: '#007aff', color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer' };

  return html`
    <div class="tunein-browser deezer-browser" style=${{ padding: '16px' }}>
      <!-- Toolbar -->
      <div class="tunein-toolbar" style=${{ display: 'flex', gap: '8px', marginBottom: '16px' }}>
        <select
          class="btn-secondary"
          style=${{ padding: '0 8px', height: '36px', borderRadius: '4px', background: '#333', color: '#fff', border: 'none' }}
          value=${searchType}
          onChange=${(e) => setSearchType(e.target.value)}
        >
          <option value="album">Albums</option>
          <option value="artist">Artisti</option>
          <option value="track">Tracce</option>
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
          setExpandedItem(null);
          setExpandedTracks([]);
        }}>Svuota</button>
      </div>

      ${statusMessage ? html`<div class="tunein-item-desc" style=${{ padding: '8px 0', color: '#aaa', fontSize: '14px' }}>${statusMessage}</div>` : null}
      ${loading ? html`<div class="loading-bar" style=${{ height: '3px', background: '#007aff', width: '100%', marginBottom: '16px' }}></div>` : null}

      <!-- Coda -->
      <div style=${{ background: '#1b1b1b', border: '1px solid #333', borderRadius: '6px', padding: '12px', marginBottom: '20px' }}>
        <div style=${{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <strong style=${{ color: '#fff' }}>Coda${queueSnapshot.tracks.length ? ` (${queueSnapshot.tracks.length})` : ''}${queueSnapshot.playing ? ' — In riproduzione' : ''}</strong>
          <div style=${{ display: 'flex', gap: '8px' }}>
            <button style=${{ ...pillBtnStyle, background: '#34c759' }} disabled=${queueSnapshot.tracks.length === 0 || queueSnapshot.playing} onClick=${playQueue}>▶ Riproduci</button>
            <button style=${{ ...pillBtnStyle, background: '#444' }} disabled=${!queueSnapshot.playing} onClick=${stopQueue}>■ Stop</button>
            <button style=${{ ...pillBtnStyle, background: '#444' }} disabled=${queueSnapshot.tracks.length === 0} onClick=${clearQueue}>Svuota</button>
          </div>
        </div>
        ${queueSnapshot.tracks.length > 0 ? html`
          <div style=${{ display: 'flex', flexDirection: 'column', gap: '6px', marginTop: '10px' }}>
            ${queueSnapshot.tracks.map((t, i) => html`
              <div key=${`q-${i}-${t.id}`} style=${{ display: 'flex', alignItems: 'center', gap: '10px', fontSize: '13px', color: (queueSnapshot.playing && i === queueSnapshot.pos) ? '#fff' : '#bbb', fontWeight: (queueSnapshot.playing && i === queueSnapshot.pos) ? 'bold' : 'normal' }}>
                ${t.cover_url ? html`<img src=${t.cover_url} style=${{ width: '28px', height: '28px', borderRadius: '3px', objectFit: 'cover' }} />` : null}
                <span style=${{ flex: '1' }}>${t.title} <span style=${{ color: '#888' }}>— ${t.artist}</span></span>
                <button style=${{ ...pillBtnStyle, background: 'transparent', padding: '2px 6px' }} disabled=${queueSnapshot.playing} onClick=${() => removeQueueItem(i)}>×</button>
              </div>
            `)}
          </div>
        ` : html`<div style=${{ color: '#888', fontSize: '13px', marginTop: '8px' }}>Nessun brano in coda — usa + su un brano, un album o un artista.</div>`}
      </div>

      <!-- Pending action -->
      ${pendingAction ? html`
        <div style=${{ padding: '12px', background: '#222', borderRadius: '6px', marginBottom: '16px' }}>
          <div style=${{ fontWeight: 'bold', marginBottom: '8px', color: '#fff' }}>
            ${pendingAction.type === 'add_queue' ? 'Aggiungi' : 'Riproduci'} "${pendingAction.item?.name || pendingAction.item?.title || ''}" su:
          </div>
          <div style=${{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
            ${Object.entries(devices || {}).map(([id, dev]) => html`
              <button class="btn btn-secondary" style=${pillBtnStyle} onClick=${() => executeAction(id, pendingAction, pendingAction.sectionItems)}>
                ${dev.info?.name || id}
              </button>
            `)}
            <button class="btn" style=${{ ...pillBtnStyle, color: '#ff4444', background: 'transparent' }} onClick=${() => setPendingAction(null)}>Annulla</button>
          </div>
        </div>
      ` : null}

      <!-- Risultati o dettagli -->
      <div class="browser-sections" style=${{ display: 'flex', flexDirection: 'column', gap: '24px' }}>
        ${expandedItem ? html`
          <!-- Dettaglio album/artista -->
          <div class="context-header" style=${{ background: '#1c1c1e', padding: '16px', borderRadius: '8px', border: '1px solid #2c2c2e' }}>
            <div style=${{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
              <div style=${{ display: 'flex', alignItems: 'center', gap: '16px' }}>
                ${expandedItem.imageUrl ? html`<img src=${expandedItem.imageUrl} style=${{ width: '64px', height: '64px', borderRadius: '6px', objectFit: 'cover' }} />` : null}
                <div>
                  <h3 style=${{ margin: '0 0 4px 0', color: '#fff', fontSize: '18px' }}>${expandedItem.name || expandedItem.title}</h3>
                  <p style=${{ margin: '0', color: '#aaa', fontSize: '14px' }}>${expandedItem.subtitle || expandedItem.artist || ''}</p>
                </div>
              </div>
              <div style=${{ display: 'flex', gap: '8px' }}>
                <button style=${stylePlayClassico} onClick=${() => handleAction('play_native', expandedItem)}>▶ Classico</button>
                <button style=${stylePlayContext} onClick=${() => handleAction('play_context', expandedItem)}>▶ Context</button>
                <button style=${styleAddQueue} onClick=${() => handleAction('add_queue', expandedItem)}>+</button>
                <button style=${styleCloseBtn} onClick=${() => { setExpandedItem(null); setExpandedTracks([]); }}>Chiudi</button>
              </div>
            </div>
            <!-- Lista tracce espulse -->
            <div class="expanded-tracks-list" style=${{ display: 'flex', flexDirection: 'column', gap: '8px', borderTop: '1px solid #333', paddingTop: '12px' }}>
              ${expandedTracks.map((track, idx) => html`
                <div class="track-row" key=${track.id} style=${{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '8px 12px', background: '#2c2c2e', borderRadius: '4px' }}>
                  <div style=${{ display: 'flex', alignItems: 'center', gap: '12px', flex: '1' }}>
                    <span style=${{ color: '#666', fontSize: '13px', width: '20px', textAlign: 'right' }}>${idx + 1}</span>
                    <span style=${{ color: '#fff', fontSize: '14px', fontWeight: '500' }}>${track.title || track.name}</span>
                  </div>
                  <div style=${{ display: 'flex', gap: '8px' }}>
                    <button style=${styleTrackPlay} onClick=${(e) => { e.stopPropagation(); handleAction('play_context', { ...track, type: 'track' }, expandedTracks); }}>▶</button>
                    <button style=${styleTrackAdd} onClick=${(e) => { e.stopPropagation(); handleAction('add_queue', { ...track, type: 'track' }, expandedTracks); }}>+</button>
                  </div>
                </div>
              `)}
            </div>
          </div>
        ` : html`
          <!-- Lista risultati -->
          ${sections.map(section => html`
            <div class="browser-section" key=${section.name}>
              <h3 style=${{ color: '#fff', borderBottom: '1px solid #333', paddingBottom: '8px', margin: '0 0 12px 0' }}>${section.name}</h3>
              <div class="browser-items" style=${{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                ${(section.items || []).map(item => {
                  if (!item) return null;
                  const currentItemType = item.type || searchType;
                  return html`
                    <div class="browser-item" key=${item.id} style=${{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '10px', background: '#1e1e1e', borderRadius: '6px' }}>
                      <div style=${{ display: 'flex', alignItems: 'center', gap: '12px', flex: '1', cursor: currentItemType !== 'track' ? 'pointer' : 'default' }}
                           onClick=${() => currentItemType !== 'track' && handleAction('explode', { ...item, type: currentItemType })}>
                        ${item.imageUrl ? html`<img src=${item.imageUrl} style=${{ width: '48px', height: '48px', borderRadius: '4px', objectFit: 'cover' }} />` : null}
                        <div style=${{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                          <span style=${{ color: '#ffffff', fontWeight: '500', fontSize: '15px' }}>${item.name || item.title}</span>
                          <span style=${{ color: '#aaaaaa', fontSize: '13px' }}>${item.subtitle || item.artist || ''}</span>
                        </div>
                      </div>
                      <div style=${{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                        ${currentItemType !== 'track' ? html`<button class="btn-secondary" style=${{ padding: '6px 10px', fontSize: '13px', background: '#333', color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer' }} onClick=${() => handleAction('explode', { ...item, type: currentItemType })}>Esplodi</button>` : null}
                        <button class="btn-primary" style=${styleTrackPlay} onClick=${(e) => { e.stopPropagation();
                          // Log per debug
                          console.log('Sezione tracce prima di play:', section.items);
                          handleAction('play_native', { ...item, type: currentItemType }, section.items); }}>▶</button>
                        <button class="btn-secondary" style=${styleTrackAdd} onClick=${(e) => { e.stopPropagation(); handleAction('add_queue', { ...item, type: currentItemType }, section.items); }}>+</button>
                      </div>
                    </div>
                  `;
                })}
              </div>
            </div>
          `)}
        `}
      </div>
    </div>
  `;
}
