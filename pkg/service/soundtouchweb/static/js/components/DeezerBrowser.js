import { h } from 'preact';
import { useState, useEffect, useCallback } from 'preact/hooks';
import htm from 'htm';
import { api } from '../api.js';

const html = htm.bind(h);

// ─── shared styles ───────────────────────────────────────────────────────────

const S = {
  pillBtn: { padding: '6px 12px', borderRadius: '4px', border: 'none', color: '#fff', cursor: 'pointer', fontSize: '13px' },
  play:    { padding: '4px 10px', background: '#34c759', color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer', fontSize: '13px' },
  add:     { padding: '4px 10px', background: '#007aff', color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer', fontSize: '13px' },
  expand:  { padding: '4px 8px',  background: '#333',    color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer', fontSize: '12px' },
};

// ─── helpers ─────────────────────────────────────────────────────────────────

// Normalises a raw Deezer API item or an already-processed item into the
// single canonical shape used throughout this component. The only field that
// matters for playback is `id` — no stream_url, no preview URL.
function normTrack({ id, title, name, artist, subtitle, album, imageUrl, cover_url }) {
  return {
    id:        Number(id),
    title:     title || name || 'Traccia sconosciuta',
    artist:    artist || subtitle || 'Artista sconosciuto',
    album:     album  || '',
    cover_url: imageUrl || cover_url || '',
  };
}

// ─── component ───────────────────────────────────────────────────────────────

export function DeezerBrowser({ devices, deviceId }) {
  // ── search state ──
  const [sections,    setSections]    = useState([]);
  const [query,       setQuery]       = useState('');
  const [searchType,  setSearchType]  = useState('album');
  const [loading,     setLoading]     = useState(false);
  const [status,      setStatus]      = useState('');

  // ── accordion: flat map "type-id" → { loading, tracks, albums } ──
  const [expanded, setExpanded] = useState({});

  // ── queue display (polled every 3 s) ──
  // Shape: { current: QueueTrack|null, upcoming: QueueTrack[], playing: bool }
  const [queue, setQueue] = useState({ current: null, upcoming: [], playing: false });

  // ── device resolution ──
  const deviceEntries   = Object.entries(devices || {}).filter(([id, dev]) => id && dev);
  const resolvedDeviceId = deviceId || (deviceEntries.length === 1 ? deviceEntries[0][0] : null);
  const [pendingAction, setPendingAction] = useState(null);

  // ── queue polling ──
  // applySnapshot normalises a raw queue data object and updates state.
  const applySnapshot = useCallback((d) => {
    setQueue({
      current:  d?.current  || null,
      upcoming: d?.upcoming || [],
      playing:  !!d?.playing,
    });
  }, []);

  // One initial REST call to hydrate the queue display when the component
  // mounts (or the target device changes). After that, all updates come via
  // the 'deezer_queue' CustomEvent dispatched by app.js from the WebSocket —
  // no polling needed.
  useEffect(() => {
    if (!resolvedDeviceId) return;
    api.deezerQueueStatus(resolvedDeviceId)
      .then(res => applySnapshot(res?.data || res))
      .catch(() => {});
  }, [resolvedDeviceId, applySnapshot]);

  useEffect(() => {
    const handler = (e) => {
      const msg = e.detail || {};
      if (msg.deviceId === resolvedDeviceId) {
        applySnapshot(msg.data);
      }
    };
    window.addEventListener('deezer_queue', handler);
    return () => window.removeEventListener('deezer_queue', handler);
  }, [resolvedDeviceId, applySnapshot]);

  // ── accordion ────────────────────────────────────────────────────────────

  function eKey(type, id) { return `${type}-${id}`; }

  async function toggleExpand(item, type) {
    const key = eKey(type, item.id);
    if (expanded[key]) {
      setExpanded(p => { const n = { ...p }; delete n[key]; return n; });
      return;
    }
    setExpanded(p => ({ ...p, [key]: { loading: true, tracks: [], albums: [] } }));
    try {
      if (type === 'album') {
        const tracks = await fetchAlbumTracks(item);
        setExpanded(p => ({ ...p, [key]: { loading: false, tracks, albums: [] } }));
      } else if (type === 'artist') {
        const res  = await api.deezerArtistDetails(item.id);
        const data = res?.data || res || {};
        const top5 = (Array.isArray(data.tracks) ? data.tracks : []).slice(0, 5).map(t =>
          normTrack({ id: t.id, title: t.title, artist: item.name, imageUrl: t.album?.cover_medium || item.imageUrl })
        );
        const albums = (Array.isArray(data.albums) ? data.albums : []).map(a => ({
          id: a.id, name: a.title, subtitle: item.name,
          imageUrl: a.cover_medium || a.cover_small || '', type: 'album',
        }));
        setExpanded(p => ({ ...p, [key]: { loading: false, tracks: top5, albums } }));
      }
    } catch (err) {
      console.error(err);
      setStatus('Errore nel caricamento dei dettagli.');
      setExpanded(p => { const n = { ...p }; delete n[key]; return n; });
    }
  }

  async function fetchAlbumTracks(albumItem) {
    const res = await api.deezerAlbumTracks(albumItem.id);
    return (res?.data || []).map(t => normTrack({
      id: t.id, title: t.title,
      artist:    albumItem.subtitle || albumItem.artist || 'Artista',
      album:     albumItem.name     || albumItem.title  || '',
      cover_url: albumItem.imageUrl || albumItem.cover_url || '',
    }));
  }

  // ── play / add actions ───────────────────────────────────────────────────

  // ▶ on anything: build a track list and REPLACE the queue (starts immediately).
  // + on anything: build a track list and APPEND to the queue.
  async function handleAction(action, item, sectionTracks = []) {
    if (!item) return;
    const type = item.type || searchType;
    const devId = resolvedDeviceId;

    let tracks = [];

    if (type === 'track') {
      if (action === 'play') {
        // "Continue from here": play this track, then the rest of the list.
        const all  = sectionTracks.filter(t => t && (t.type === 'track' || !t.type));
        const idx  = all.findIndex(t => String(t.id) === String(item.id));
        const from = idx >= 0 ? all.slice(idx) : [item];
        tracks = from.map(t => normTrack(t));
      } else {
        tracks = [normTrack(item)];
      }
    } else if (type === 'album') {
      setLoading(true);
      try { tracks = await fetchAlbumTracks(item); }
      catch (e) { console.error(e); setStatus("Impossibile caricare le tracce."); setLoading(false); return; }
      finally   { setLoading(false); }
    } else if (type === 'artist') {
      setLoading(true);
      try {
        const res = await api.deezerArtistTracklist(item.id);
        tracks = (res?.data || res || []).map(t => normTrack({
          id: t.id, title: t.title, artist: item.name,
          imageUrl: t.album?.cover_medium || t.album?.cover_small || item.imageUrl || '',
        }));
      } catch (e) { console.error(e); setStatus("Impossibile caricare la tracklist."); setLoading(false); return; }
      finally     { setLoading(false); }
    }

    if (!tracks.length) { setStatus('Nessuna traccia valida.'); return; }

    const task = { action, item: { ...item, type }, tracks };
    if (devId) { await executeTask(devId, task); }
    else       { setPendingAction(task); }
  }

  async function executeTask(devId, task) {
    const { action, item, tracks } = task;
    setLoading(true);
    try {
      if (action === 'play') {
        await api.deezerQueueReplace(devId, tracks);
        setStatus(`In coda: ${item.name || item.title}`);
      } else {
        await api.deezerQueueAdd(devId, tracks);
        setStatus(`Aggiunto: ${item.name || item.title}`);
      }
      setTimeout(() => setStatus(''), 2500);
    } catch (err) {
      console.error(err);
      setStatus(`Errore: ${err.message}`);
    } finally {
      setLoading(false);
      setPendingAction(null);
    }
  }

  // ── queue controls ───────────────────────────────────────────────────────

  async function stopQueue() {
    if (!resolvedDeviceId) return;
    try { await api.deezerQueueStop(resolvedDeviceId); }
    catch (e) { console.error(e); }
  }

  async function removeUpcoming(index) {
    if (!resolvedDeviceId) return;
    try { await api.deezerQueueRemove(resolvedDeviceId, index); }
    catch (e) { console.error(e); }
  }

  // ── search ───────────────────────────────────────────────────────────────

  async function search(q, type) {
    if (!q?.trim()) return;
    setLoading(true); setStatus('Ricerca su Deezer...'); setSections([]); setExpanded({});
    try {
      const res  = await api.deezerSearch(q, type);
      const list = res?.data;
      if (!Array.isArray(list) || !list.length) { setStatus('Nessun risultato.'); return; }
      setStatus(`${list.length} risultati:`);
      setSections([{ name: `${type[0].toUpperCase() + type.slice(1)} Risultati`, items: mapItems(list, type) }]);
    } catch (e) { console.error(e); setStatus('Errore nella ricerca.'); }
    finally     { setLoading(false); }
  }

  function mapItems(list, defaultType) {
    return list.filter(Boolean).filter(i => !(( i.type || defaultType) === 'artist' && i.nb_album === 0)).map(item => {
      const type = item.type || defaultType;
      if (type === 'track') return {
        ...normTrack({ id: item.id, title: item.title, artist: item.artist?.name, imageUrl: item.album?.cover_medium || item.album?.cover_small }),
        type: 'track',
      };
      if (type === 'album') return {
        id: item.id, type: 'album',
        name: item.title || '', subtitle: item.artist?.name || '',
        imageUrl: item.cover_medium || item.cover_small || '',
      };
      // artist
      return {
        id: item.id, type: 'artist',
        name: item.name || '', subtitle: item.nb_album != null ? `${item.nb_album} Album` : 'Artista',
        imageUrl: item.picture_medium || item.picture_small || '',
      };
    });
  }

  // ── render helpers ───────────────────────────────────────────────────────

  // Renders one item row inline-accordion style. Expanding never replaces the
  // surrounding list — it just inserts content below the row.
  function renderRow(item, contextList, depth = 0) {
    if (!item) return null;
    const type = item.type || searchType;
    const key  = eKey(type, item.id);
    const isExpandable = type === 'album' || type === 'artist';
    const entry  = expanded[key];
    const isOpen = !!entry;

    const bg = depth === 0 ? '#1e1e1e' : '#181818';
    const ml = depth * 24;

    return html`<div key=${key}>
      <!-- item row -->
      <div style=${{ display:'flex', alignItems:'center', gap:'10px', padding:'8px 10px', background:bg, borderRadius:'6px', marginLeft:`${ml}px` }}>
        ${item.imageUrl ? html`<img src=${item.imageUrl} style=${{ width:'44px', height:'44px', borderRadius:'4px', objectFit:'cover', flexShrink:0 }} />` : null}
        <div style=${{ flex:1, minWidth:0, cursor: isExpandable ? 'pointer' : 'default' }} onClick=${() => isExpandable && toggleExpand(item, type)}>
          <div style=${{ color:'#fff', fontWeight:500, fontSize:'14px', whiteSpace:'nowrap', overflow:'hidden', textOverflow:'ellipsis' }}>${item.name || item.title}</div>
          <div style=${{ color:'#888', fontSize:'12px' }}>${item.subtitle || item.artist || ''}</div>
        </div>
        <div style=${{ display:'flex', gap:'6px', alignItems:'center', flexShrink:0 }}>
          ${isExpandable ? html`<button style=${S.expand} onClick=${() => toggleExpand(item, type)}>${isOpen ? '▾' : '▸'}</button>` : null}
          <button style=${S.play} onClick=${(e) => { e.stopPropagation(); handleAction('play', { ...item, type }, contextList); }} title=${type === 'artist' ? 'Top 50' : 'Riproduci'}>▶</button>
          <button style=${S.add}  onClick=${(e) => { e.stopPropagation(); handleAction('add',  { ...item, type }, contextList); }} title="Aggiungi in coda">+</button>
        </div>
      </div>

      <!-- accordion body -->
      ${isOpen ? html`
        <div style=${{ marginLeft:`${ml + 20}px`, borderLeft:'2px solid #333', paddingLeft:'10px', marginTop:'4px', marginBottom:'8px' }}>
          ${entry.loading ? html`<div style=${{ color:'#888', padding:'8px', fontSize:'13px' }}>Caricamento...</div>` : html`
            ${type === 'artist' && entry.tracks.length ? html`
              <div style=${{ color:'#aaa', fontSize:'11px', fontWeight:600, letterSpacing:'.05em', padding:'4px 0 6px' }}>TOP 5</div>
              ${entry.tracks.map((t, i) => html`
                <div key=${t.id} style=${{ display:'flex', alignItems:'center', gap:'8px', padding:'5px 8px', background:'#252525', borderRadius:'4px', marginBottom:'4px' }}>
                  <span style=${{ color:'#888', fontSize:'12px', width:'16px', textAlign:'right', flexShrink:0 }}>${i + 1}</span>
                  <span style=${{ flex:1, color:'#fff', fontSize:'13px', whiteSpace:'nowrap', overflow:'hidden', textOverflow:'ellipsis' }}>${t.title}</span>
                  <button style=${S.play} onClick=${(e) => { e.stopPropagation(); handleAction('play', { ...t, type:'track' }, entry.tracks); }}>▶</button>
                  <button style=${S.add}  onClick=${(e) => { e.stopPropagation(); handleAction('add',  { ...t, type:'track' }, entry.tracks); }}>+</button>
                </div>
              `)}
            ` : null}
            ${type === 'artist' && entry.albums.length ? html`
              <div style=${{ color:'#aaa', fontSize:'11px', fontWeight:600, letterSpacing:'.05em', padding:'10px 0 6px' }}>ALBUM</div>
              ${entry.albums.map(a => renderRow(a, entry.albums, depth + 1))}
            ` : null}
            ${type === 'album' ? entry.tracks.map((t, i) => html`
              <div key=${t.id} style=${{ display:'flex', alignItems:'center', gap:'8px', padding:'5px 8px', background:'#252525', borderRadius:'4px', marginBottom:'4px' }}>
                <span style=${{ color:'#888', fontSize:'12px', width:'16px', textAlign:'right', flexShrink:0 }}>${i + 1}</span>
                <span style=${{ flex:1, color:'#fff', fontSize:'13px', whiteSpace:'nowrap', overflow:'hidden', textOverflow:'ellipsis' }}>${t.title}</span>
                <button style=${S.play} onClick=${(e) => { e.stopPropagation(); handleAction('play', { ...t, type:'track' }, entry.tracks); }}>▶</button>
                <button style=${S.add}  onClick=${(e) => { e.stopPropagation(); handleAction('add',  { ...t, type:'track' }, entry.tracks); }}>+</button>
              </div>
            `) : null}
          `}
        </div>
      ` : null}
    </div>`;
  }

  // ─── render ──────────────────────────────────────────────────────────────

  return html`
    <div class="tunein-browser deezer-browser" style=${{ padding:'16px' }}>

      <!-- toolbar -->
      <div style=${{ display:'flex', gap:'8px', marginBottom:'16px' }}>
        <select
          style=${{ padding:'0 8px', height:'36px', borderRadius:'4px', background:'#333', color:'#fff', border:'none' }}
          value=${searchType}
          onChange=${(e) => setSearchType(e.target.value)}
        >
          <option value="album">Album</option>
          <option value="artist">Artisti</option>
          <option value="track">Tracce</option>
        </select>
        <input
          class="tunein-search-input"
          style=${{ flex:1, padding:'0 12px', height:'36px', borderRadius:'4px', background:'#222', color:'#fff', border:'1px solid #444' }}
          placeholder="Cerca su Deezer..."
          value=${query}
          onInput=${(e) => setQuery(e.target.value)}
          onKeyDown=${(e) => e.key === 'Enter' && search(query, searchType)}
        />
        <button style=${{ ...S.pillBtn, background:'#007aff', height:'36px', padding:'0 16px' }} onClick=${() => search(query, searchType)}>Cerca</button>
        <button style=${{ ...S.pillBtn, background:'#444',    height:'36px', padding:'0 16px' }} onClick=${() => { setQuery(''); setStatus(''); setSections([]); setExpanded({}); }}>Svuota</button>
      </div>

      ${status  ? html`<div style=${{ color:'#aaa', fontSize:'13px', marginBottom:'8px' }}>${status}</div>` : null}
      ${loading ? html`<div class="loading-bar" style=${{ height:'3px', background:'#007aff', width:'100%', marginBottom:'12px' }}></div>` : null}

      <!-- ── queue panel ── -->
      <div style=${{ background:'#1b1b1b', border:'1px solid #2a2a2a', borderRadius:'8px', padding:'12px', marginBottom:'20px' }}>
        <!-- header -->
        <div style=${{ display:'flex', justifyContent:'space-between', alignItems:'center', marginBottom: queue.current || queue.upcoming.length ? '10px' : '0' }}>
          <span style=${{ color:'#fff', fontWeight:600, fontSize:'14px' }}>
            Coda${queue.upcoming.length ? ` · ${queue.upcoming.length} in attesa` : ''}
          </span>
          ${queue.playing ? html`<button style=${{ ...S.pillBtn, background:'#555' }} onClick=${stopQueue}>■ Stop</button>` : null}
        </div>

        <!-- currently playing -->
        ${queue.current ? html`
          <div style=${{ display:'flex', alignItems:'center', gap:'10px', padding:'8px 10px', background:'#262626', borderRadius:'6px', marginBottom: queue.upcoming.length ? '8px' : '0', border:'1px solid #3a3a3a' }}>
            ${queue.current.cover_url ? html`<img src=${queue.current.cover_url} style=${{ width:'40px', height:'40px', borderRadius:'4px', objectFit:'cover' }} />` : null}
            <div style=${{ flex:1, minWidth:0 }}>
              <div style=${{ color:'#34c759', fontSize:'11px', fontWeight:600, marginBottom:'2px' }}>▶ IN RIPRODUZIONE</div>
              <div style=${{ color:'#fff', fontSize:'14px', fontWeight:500, whiteSpace:'nowrap', overflow:'hidden', textOverflow:'ellipsis' }}>${queue.current.title}</div>
              <div style=${{ color:'#888', fontSize:'12px' }}>${queue.current.artist}</div>
            </div>
          </div>
        ` : !queue.playing ? html`
          <div style=${{ color:'#555', fontSize:'13px' }}>Nessuna traccia in coda — usa ▶ o + dai risultati.</div>
        ` : null}

        <!-- upcoming -->
        ${queue.upcoming.map((t, i) => html`
          <div key=${`upc-${i}-${t.id}`} style=${{ display:'flex', alignItems:'center', gap:'10px', padding:'6px 10px', background: i % 2 === 0 ? '#1e1e1e' : '#222', borderRadius:'4px', marginBottom:'4px' }}>
            <span style=${{ color:'#555', fontSize:'12px', width:'18px', textAlign:'right', flexShrink:0 }}>${i + 1}</span>
            ${t.cover_url ? html`<img src=${t.cover_url} style=${{ width:'32px', height:'32px', borderRadius:'3px', objectFit:'cover' }} />` : null}
            <div style=${{ flex:1, minWidth:0 }}>
              <div style=${{ color:'#ddd', fontSize:'13px', whiteSpace:'nowrap', overflow:'hidden', textOverflow:'ellipsis' }}>${t.title}</div>
              <div style=${{ color:'#666', fontSize:'11px' }}>${t.artist}</div>
            </div>
            <button style=${{ ...S.pillBtn, background:'transparent', color:'#666', padding:'2px 6px', fontSize:'16px' }} onClick=${() => removeUpcoming(i)}>×</button>
          </div>
        `)}
      </div>

      <!-- ── device picker (multi-device) ── -->
      ${pendingAction ? html`
        <div style=${{ padding:'12px', background:'#222', borderRadius:'6px', marginBottom:'16px' }}>
          <div style=${{ color:'#fff', fontWeight:600, marginBottom:'8px' }}>
            ${pendingAction.action === 'add' ? 'Aggiungi' : 'Riproduci'} "${pendingAction.item?.name || pendingAction.item?.title}" su:
          </div>
          <div style=${{ display:'flex', gap:'8px', flexWrap:'wrap' }}>
            ${deviceEntries.map(([id, dev]) => html`
              <button style=${{ ...S.pillBtn, background:'#444' }} onClick=${() => executeTask(id, pendingAction)}>
                ${dev.info?.name || id}
              </button>
            `)}
            <button style=${{ ...S.pillBtn, background:'transparent', color:'#f44' }} onClick=${() => setPendingAction(null)}>Annulla</button>
          </div>
        </div>
      ` : null}

      <!-- ── search results / accordion ── -->
      ${sections.map(section => html`
        <div key=${section.name} style=${{ marginBottom:'24px' }}>
          <h3 style=${{ color:'#fff', borderBottom:'1px solid #333', paddingBottom:'8px', margin:'0 0 10px 0', fontSize:'15px' }}>${section.name}</h3>
          <div style=${{ display:'flex', flexDirection:'column', gap:'6px' }}>
            ${section.items.map(item => renderRow(item, section.items, 0))}
          </div>
        </div>
      `)}
    </div>
  `;
}
