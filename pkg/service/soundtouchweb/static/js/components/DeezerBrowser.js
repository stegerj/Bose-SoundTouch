import { h } from 'preact';
import { useState, useEffect, useCallback } from 'preact/hooks';
import htm from 'htm';
import { api } from '../api.js';

const html = htm.bind(h);

// ─── shared styles (defined outside to avoid re-allocation on render) ────────

const S = {
  pillBtn: { padding: '6px 12px', borderRadius: '4px', border: 'none', color: '#fff', cursor: 'pointer', fontSize: '13px', display: 'inline-flex', alignItems: 'center', justifyContent: 'center' },
  play:    { padding: '4px 10px', background: '#34c759', color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer', fontSize: '13px' },
  add:     { padding: '4px 10px', background: '#007aff', color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer', fontSize: '13px' },
  expand:  { padding: '4px 8px',  background: '#333',    color: '#fff', border: 'none', borderRadius: '4px', cursor: 'pointer', fontSize: '12px' },

  // Clean Row container bindings for optimized GC on embedded systems
  rowOuterStyle: (depth, bg) => ({
    display: 'flex',
    alignItems: 'center',
    gap: '10px',
    padding: '8px 10px',
    background: bg,
    borderRadius: '6px',
    marginLeft: `${depth * 24}px`
  }),
  rowInnerStyle: (isExpandable) => ({
    flex: 1,
    minWidth: 0,
    cursor: isExpandable ? 'pointer' : 'default'
  }),
  rowTextTitle: { color: '#fff', fontWeight: 500, fontSize: '14px', overflowWrap: 'anywhere', wordBreak: 'break-word' },
  rowTextSubtitle: { color: '#888', fontSize: '12px' },
  rowActions: { display: 'flex', gap: '6px', alignItems: 'center', flexShrink: 0 },
  trackItem: { display: 'flex', alignItems: 'center', gap: '8px', padding: '5px 8px', background: '#252525', borderRadius: '4px', marginBottom: '4px' },
  trackNumber: { color: '#888', fontSize: '12px', width: '16px', textAlign: 'right', flexShrink: 0 },
  trackTitle: { flex: 1, color: '#fff', fontSize: '13px', overflowWrap: 'anywhere', wordBreak: 'break-word' },
};

// Status message timeout in milliseconds
const STATUS_TIMEOUT = 4000;
const SUCCESS_TIMEOUT = 2500;

// Normalises any track-like object into the canonical shape.
// Ensures BOTH cover_url and imageUrl are set to fix the search/track rendering bug.
function normTrack({ id, title, name, artist, subtitle, imageUrl, cover_url, duration }) {
  const finalId = id ? Number(id) : 0;
  const imageSource = imageUrl || cover_url || '';
  return {
    id:        Number.isNaN(finalId) ? 0 : finalId,
    title:     title || name || 'Unknown track',
    artist:    artist || subtitle || 'Unknown artist',
    cover_url: imageSource,
    imageUrl:  imageSource, // Fallback for components scanning this property specifically
    duration:  duration ? Number(duration) : 0, // Track duration in seconds from Deezer
  };
}

// Fetches top-5 tracks + full album list + related artists in parallel.
async function fetchArtistData(artist) {
  const [detailsRes, relatedRes] = await Promise.all([
    api.deezerArtistDetails(artist.id),
    api.deezerArtistRelated(artist.id).catch(() => ({ data: [] })),
  ]);
  const data = detailsRes?.data || detailsRes || {};
  return {
    tracks: (Array.isArray(data.tracks) ? data.tracks : []).slice(0, 5).map(t =>
      normTrack({ id: t.id, title: t.title, artist: artist.name,
        imageUrl: t.album?.cover_medium || t.album?.cover_small || '' })),
    albums: (Array.isArray(data.albums) ? data.albums : []).map(a => ({
      id: a.id, name: a.title, subtitle: artist.name,
      imageUrl: a.cover_medium || a.cover_small || '', type: 'album',
    })),
    related: (Array.isArray(relatedRes?.data) ? relatedRes.data : []).map(a => ({
      id: a.id, name: a.name || '',
      subtitle: a.nb_album != null ? `${a.nb_album} album` : 'Artist',
      imageUrl: a.picture_medium || a.picture_small || '', type: 'artist',
    })),
  };
}

export function DeezerBrowser({ devices, deviceId }) {
  // ── search state ──
  const [sections,    setSections]    = useState([]);
  const [query,       setQuery]       = useState('');
  const [searchType,  setSearchType]  = useState('album');
  const [loading,     setLoading]     = useState(false);
  const [status,      setStatus]      = useState('');

  // ── accordion (used both in search results and in artist pages for albums) ──
  const [expanded, setExpanded] = useState({});

  // ── artist page navigation ──
  const [artistPage,    setArtistPage]    = useState(null);
  const [artistHistory, setArtistHistory] = useState([]);

  // ── queue ──
  const [queue, setQueue] = useState({ current: null, upcoming: [], playing: false, paused: false });
  const [queueOpen, setQueueOpen] = useState(true);
  const deviceEntries    = Object.entries(devices || {}).filter(([id, dev]) => id && dev);
  const resolvedDeviceId = deviceId || (deviceEntries.length === 1 ? deviceEntries[0][0] : null);
  const [pendingAction, setPendingAction] = useState(null);

  const applySnapshot = useCallback((d) => {
    setQueue({
      current:  d?.current  || null,
      upcoming: d?.upcoming || [],
      playing:  !!d?.playing,
      paused:   !!d?.paused,
    });
  }, []);

  useEffect(() => {
    if (!resolvedDeviceId) return;
    api.deezerQueueStatus(resolvedDeviceId)
      .then(res => applySnapshot(res?.data || res))
      .catch(() => {});
  }, [resolvedDeviceId, applySnapshot]);

  useEffect(() => {
    const handler = (e) => {
      const msg = e.detail || {};
      if (msg.deviceId === resolvedDeviceId) applySnapshot(msg.data);
    };
    window.addEventListener('deezer_queue', handler);
    return () => window.removeEventListener('deezer_queue', handler);
  }, [resolvedDeviceId, applySnapshot]);

  // ── artist page navigation ────────────────────────────────────────────────

  async function showArtistPage(artist) {
    setArtistHistory(prev => artistPage ? [...prev, artistPage] : prev);
    setArtistPage({ artist, tracks: [], albums: [], related: [], loading: true });
    setExpanded({});

    try {
      const { tracks: top5, albums, related } = await fetchArtistData(artist);
      setArtistPage({ artist, tracks: top5, albums, related, loading: false });
    } catch (err) {
      console.error('[artist page]', err);
      setStatus("Error loading artist.");
      setTimeout(() => setStatus(''), STATUS_TIMEOUT);
      goBack();
    }
  }

  function goBack() {
    if (artistHistory.length > 0) {
      setArtistPage(artistHistory[artistHistory.length - 1]);
      setArtistHistory(prev => prev.slice(0, -1));
    } else {
      setArtistPage(null);
    }
    setExpanded({});
  }

  // ── accordion (albums only — artists navigate, never expand inline) ───────

  function eKey(type, id) { return `${type}-${id}`; }

  async function toggleExpand(item, type) {
    const key = eKey(type, item.id);
    if (expanded[key]) {
      setExpanded(p => {
        const { [key]: _, ...rest } = p;
        return rest;
      });
      return;
    }
    setExpanded(p => ({ ...p, [key]: { loading: true, tracks: [], albums: [], related: [] } }));
    try {
      if (type === 'album') {
        const tracks = await fetchAlbumTracks(item);
        setExpanded(p => ({ ...p, [key]: { loading: false, tracks, albums: [], related: [] } }));
      } else if (type === 'artist') {
        const { tracks: top5, albums, related } = await fetchArtistData(item);
        setExpanded(p => ({ ...p, [key]: { loading: false, tracks: top5, albums, related } }));
      }
    } catch (err) {
      console.error('[toggle expand]', err);
      setStatus('Error loading details.');
      setTimeout(() => setStatus(''), STATUS_TIMEOUT);
      setExpanded(p => {
        const { [key]: _, ...rest } = p;
        return rest;
      });
    }
  }

  async function fetchAlbumTracks(albumItem) {
    const res = await api.deezerAlbumTracks(albumItem.id);
    return (res?.data || []).map(t => normTrack({
      id: t.id, title: t.title,
      artist:    albumItem.subtitle || albumItem.artist || 'Artist',
      album:     albumItem.name     || albumItem.title  || '',
      cover_url: albumItem.imageUrl || albumItem.cover_url || '',
    }));
  }

  // ── play / add actions ───────────────────────────────────────────────────

  async function handleAction(action, item, sectionTracks = []) {
    if (!item) return;
    const type  = item.type || searchType;
    const devId = resolvedDeviceId;
    let tracks  = [];

    // Native play modes: bypass queue for better preset support
    if (action === 'play') {
      if (type === 'album') {
        setLoading(true);
        try {
          await api.deezerPlayAlbum(devId, item.id, item.name || item.title);
          setStatus(`Playing album: ${item.name || item.title}`);
          setTimeout(() => setStatus(''), SUCCESS_TIMEOUT);
        } catch (e) {
          console.error('[play album]', e);
          setStatus(`Error: ${e.message || "Cannot play album"}`);
          setTimeout(() => setStatus(''), STATUS_TIMEOUT);
        } finally {
          setLoading(false);
        }
        return;
      }
      if (type === 'track') {
        setLoading(true);
        try {
          await api.deezerPlayTrack(devId, item.id, item.title, item.artist);
          setStatus(`Playing: ${item.title}`);
          setTimeout(() => setStatus(''), SUCCESS_TIMEOUT);
        } catch (e) {
          console.error('[play track]', e);
          setStatus(`Error: ${e.message || "Cannot play track"}`);
          setTimeout(() => setStatus(''), STATUS_TIMEOUT);
        } finally {
          setLoading(false);
        }
        return;
      }
      if (type === 'artist') {
        setLoading(true);
        try {
          await api.deezerPlayArtist(devId, item.id, item.name);
          setStatus(`Playing: ${item.name}`);
          setTimeout(() => setStatus(''), SUCCESS_TIMEOUT);
        } catch (e) {
          console.error('[play artist]', e);
          setStatus(`Error: ${e.message || "Cannot play artist"}`);
          setTimeout(() => setStatus(''), STATUS_TIMEOUT);
        } finally {
          setLoading(false);
        }
        return;
      }
    }

    // Add to queue: fetch tracks and add to queue
    if (type === 'track') {
      tracks = [normTrack(item)];
    } else if (type === 'album') {
      setLoading(true);
      try   { tracks = await fetchAlbumTracks(item); }
      catch (e) {
        console.error('[handle action album]', e);
        setStatus('Cannot load tracks.');
        setTimeout(() => setStatus(''), STATUS_TIMEOUT);
        setLoading(false);
        return;
      }
      finally   { setLoading(false); }
    } else if (type === 'artist') {
      setLoading(true);
      try {
        const res = await api.deezerArtistTracklist(item.id);
        tracks = (res?.data || res || []).map(t => normTrack({
          id: t.id, title: t.title, artist: item.name,
          imageUrl: t.album?.cover_medium || t.album?.cover_small || item.imageUrl || '',
        }));
      } catch (e) {
        console.error('[handle action artist]', e);
        setStatus('Cannot load tracklist.');
        setTimeout(() => setStatus(''), STATUS_TIMEOUT);
        setLoading(false);
        return;
      }
      finally     { setLoading(false); }
    }

    if (!tracks.length) {
      setStatus('No valid tracks.');
      setTimeout(() => setStatus(''), STATUS_TIMEOUT);
      return;
    }
    const task = { action, item: { ...item, type }, tracks };
    if (devId) { await executeTask(devId, task); }
    else       { setPendingAction(task); }
  }

  async function executeTask(devId, task) {
    const { action, item, tracks } = task;
    setLoading(true);
    try {
      // Only add to queue - play actions now use native endpoints
      await api.deezerQueueAdd(devId, tracks);
      const trackCount = tracks.length;
      const itemName = item.name || item.title;
      if (item.type === 'artist') {
        setStatus(`Added ${trackCount} tracks by ${itemName} to queue`);
      } else if (item.type === 'album') {
        setStatus(`Added ${trackCount} tracks from ${itemName} to queue`);
      } else {
        setStatus(`Added ${itemName} to queue`);
      }
      setTimeout(() => setStatus(''), SUCCESS_TIMEOUT);
    } catch (err) {
      console.error('[execute task]', err);
      setStatus(`Error: ${err.message || "Cannot complete action"}`);
      setTimeout(() => setStatus(''), STATUS_TIMEOUT);
    } finally {
      setLoading(false);
      setPendingAction(null);
    }
  }

  // ── queue controls ───────────────────────────────────────────────────────

  async function stopQueue()      { if (!resolvedDeviceId) return; try { await api.deezerQueueStop(resolvedDeviceId);  } catch(e){console.error('[queue stop]', e);} }
  async function playQueue()      { if (!resolvedDeviceId) return; try { await api.deezerQueuePlay(resolvedDeviceId);  } catch(e){console.error('[queue play]', e);} }
  async function skipTrack()      { if (!resolvedDeviceId) return; try { await api.deezerQueueNext(resolvedDeviceId);  } catch(e){console.error('[queue skip]', e);} }
  async function clearQueue()     { if (!resolvedDeviceId) return; try { await api.deezerQueueClear(resolvedDeviceId); } catch(e){console.error('[queue clear]', e);} }
  async function removeUpcoming(i){ if (!resolvedDeviceId) return; try { await api.deezerQueueRemove(resolvedDeviceId, i); } catch(e){console.error('[queue remove]', e);} }

  // ── search ───────────────────────────────────────────────────────────────

  async function search(q, type) {
    if (!q?.trim()) return;
    setLoading(true); setStatus('Searching Deezer...'); setSections([]); setExpanded({});
    setArtistPage(null); setArtistHistory([]);
    try {
      const res  = await api.deezerSearch(q, type);
      const list = res?.data;
      if (!Array.isArray(list) || !list.length) {
        setStatus('No results.');
        setTimeout(() => setStatus(''), STATUS_TIMEOUT);
        return;
      }
      setStatus(`${list.length} results:`);
      setSections([{ name: `${type[0].toUpperCase() + type.slice(1)} Results`, items: mapItems(list, type) }]);
    } catch (e) {
      console.error('[search]', e);
      setStatus('Search error.');
      setTimeout(() => setStatus(''), STATUS_TIMEOUT);
    }
    finally     { setLoading(false); }
  }

  function mapItems(list, defaultType) {
    return list.filter(Boolean)
      .filter(i => !((i.type || defaultType) === 'artist' && i.nb_album === 0))
      .map(item => {
        const type = item.type || defaultType;
        if (type === 'track') return {
          ...normTrack({ id: item.id, title: item.title, artist: item.artist?.name,
            imageUrl: item.album?.cover_medium || item.album?.cover_small }),
          type: 'track',
        };
        if (type === 'album') return {
          id: item.id, type: 'album',
          name: item.title || '', subtitle: item.artist?.name || '',
          imageUrl: item.cover_medium || item.cover_small || '',
        };
        return {
          id: item.id, type: 'artist',
          name: item.name || '', subtitle: item.nb_album != null ? `${item.nb_album} Album` : 'Artist',
          imageUrl: item.picture_medium || item.picture_small || '',
        };
      });
  }

  // ── render helpers ───────────────────────────────────────────────────────

  function renderTrackItem(track, index, contextList, background = '#252525') {
    return html`
      <div key=${track.id} style=${{ ...S.trackItem, background }}>
        <span style=${S.trackNumber}>${index + 1}</span>
        <span style=${S.trackTitle}>${track.title}</span>
        <button style=${S.play} onClick=${(e) => { e.stopPropagation(); handleAction('play', { ...track, type:'track' }, contextList); }} title="Play now">▶</button>
        <button style=${S.add}  onClick=${(e) => { e.stopPropagation(); handleAction('add',  { ...track, type:'track' }, contextList); }} title="Add to queue">+</button>
      </div>
    `;
  }

  function renderRow(item, contextList, depth = 0) {
    if (!item) return null;
    const type = item.type || searchType;
    const key  = eKey(type, item.id);

    const isExpandable = type === 'album' || type === 'artist';
    const entry  = expanded[key];
    const isOpen = !!entry;
    const bg     = depth === 0 ? '#1e1e1e' : '#181818';

    // Fixed: Defensive assignment to support both cover_url and imageUrl
    const artworkUrl = item.imageUrl || item.cover_url || '';

    return html`<div key=${key}>
      <div style=${S.rowOuterStyle(depth, bg)}>
        ${artworkUrl ? html`<img src=${artworkUrl} style=${{ width:'44px', height:'44px',
            borderRadius: type === 'artist' ? '50%' : '4px', objectFit:'cover', flexShrink:0 }} />` : null}
        <div style=${S.rowInnerStyle(isExpandable)}
             onClick=${() => isExpandable && toggleExpand(item, type)}>
          <div style=${S.rowTextTitle}>${item.name || item.title}</div>
          <div style=${S.rowTextSubtitle}>${item.subtitle || item.artist || ''}</div>
        </div>
        <div style=${S.rowActions}>
          ${isExpandable ? html`<button style=${S.expand} onClick=${() => toggleExpand(item, type)}>${isOpen ? '▾' : '▸'}</button>` : null}
          <button style=${S.play} onClick=${(e) => { e.stopPropagation(); handleAction('play', { ...item, type }, contextList); }}
                  title=${type === 'album' ? 'Play album (native mode)' : type === 'artist' ? 'Play top tracks' : 'Play now'}>${type === 'artist' ? '▶ Top 50' : '▶'}</button>
          <button style=${S.add}  onClick=${(e) => { e.stopPropagation(); handleAction('add', { ...item, type }, contextList); }} title="Add to queue">+</button>
        </div>
      </div>

      ${isOpen ? html`
        <div style=${{ marginLeft:`${depth * 24 + 20}px`, borderLeft:'2px solid #333',
                       paddingLeft:'10px', marginTop:'4px', marginBottom:'8px' }}>
          ${entry.loading
            ? html`<div style=${{ color:'#888', padding:'8px', fontSize:'13px' }}>Loading...</div>`
            : html`
              ${type === 'artist' && entry.tracks?.length ? html`
                <div style=${{ color:'#aaa', fontSize:'11px', fontWeight:600, letterSpacing:'.05em', padding:'4px 0 6px' }}>TOP 5</div>
                ${entry.tracks.map((t, i) => renderTrackItem(t, i, entry.tracks))}
              ` : null}
              ${type === 'artist' && entry.albums?.length ? html`
                <div style=${{ color:'#aaa', fontSize:'11px', fontWeight:600, letterSpacing:'.05em', padding:'10px 0 6px' }}>ALBUM</div>
                ${entry.albums.map(a => renderRow(a, entry.albums, depth + 1))}
              ` : null}
              ${type === 'artist' && entry.related?.length ? html`
                <div style=${{ color:'#aaa', fontSize:'11px', fontWeight:600, letterSpacing:'.05em', padding:'10px 0 6px' }}>RELATED ARTISTS</div>
                ${entry.related.map(a => renderRelatedArtistRow(a))}
              ` : null}
              ${type === 'album' ? entry.tracks.map((t, i) => renderTrackItem(t, i, entry.tracks)) : null}
            `}
        </div>
      ` : null}
    </div>`;
  }

  function renderRelatedArtistRow(artist) {
    const artworkUrl = artist.imageUrl || artist.cover_url || '';
    return html`
      <div key=${artist.id} style=${{ display:'flex', alignItems:'center', gap:'8px', padding:'6px 8px',
                                      background:'#252525', borderRadius:'4px', marginBottom:'4px' }}>
        ${artworkUrl ? html`<img src=${artworkUrl} style=${{ width:'36px', height:'36px',
            borderRadius:'50%', objectFit:'cover', flexShrink:0 }} />` : null}
        <div style=${{ flex:1, minWidth:0, cursor:'pointer' }} onClick=${() => showArtistPage(artist)}>
          <div style=${{ color:'#fff', fontSize:'13px', fontWeight:500, whiteSpace:'nowrap',
                         overflow:'hidden', textOverflow:'ellipsis' }}>${artist.name}</div>
          <div style=${{ color:'#666', fontSize:'11px' }}>${artist.subtitle || ''}</div>
        </div>
        <button style=${S.play} onClick=${(e) => { e.stopPropagation(); handleAction('play', { ...artist, type:'artist' }, []); }}>▶</button>
        <button style=${S.add}  onClick=${(e) => { e.stopPropagation(); handleAction('add',  { ...artist, type:'artist' }, []); }}>+</button>
        <button style=${{ ...S.expand, background:'#2a2a2a', color:'#888', fontSize:'16px', padding:'2px 8px' }}
                title="Open artist" onClick=${() => showArtistPage(artist)}>›</button>
      </div>
    `;
  }

  function renderArtistPage() {
    const { artist, tracks, albums, related, loading: pageLoading } = artistPage;

    return html`
      <div>
        <!-- breadcrumb / back -->
        <div style=${{ display:'flex', alignItems:'center', gap:'10px', marginBottom:'14px' }}>
          <button style=${{ ...S.pillBtn, background:'#333' }} onClick=${goBack}>← Back</button>
          ${artistHistory.length > 0 ? html`
            <span style=${{ color:'#666', fontSize:'12px' }}>
              ${artistHistory.map(e => e.artist.name).join(' › ')} ›
            </span>
          ` : null}
          <span style=${{ color:'#fff', fontWeight:600 }}>${artist.name}</span>
        </div>

        <!-- artist header -->
        <div style=${{ display:'flex', alignItems:'center', gap:'16px', padding:'14px',
                       background:'#1e1e1e', borderRadius:'8px', marginBottom:'16px' }}>
          ${artist.imageUrl ? html`<img src=${artist.imageUrl} style=${{ width:'72px', height:'72px',
              borderRadius:'50%', objectFit:'cover', flexShrink:0 }} />` : null}
          <div style=${{ flex:1, minWidth:0 }}>
            <div style=${{ color:'#fff', fontSize:'20px', fontWeight:700 }}>${artist.name}</div>
            <div style=${{ color:'#888', fontSize:'13px' }}>${artist.subtitle || ''}</div>
          </div>
          <button style=${{ ...S.play, padding:'8px 16px' }}
                  onClick=${() => handleAction('play', { ...artist, type:'artist' }, [])}>▶ Top 50</button>
          <button style=${{ ...S.add, padding:'8px 16px' }}
                  onClick=${() => handleAction('add', { ...artist, type:'artist' }, [])}>+ Queue</button>
        </div>

        ${pageLoading ? html`<div class="loading-bar" style=${{ height:'3px', background:'#007aff', width:'100%', marginBottom:'12px' }}></div>` : null}

        <!-- top 5 -->
        ${tracks.length ? html`
          <div style=${{ color:'#aaa', fontSize:'11px', fontWeight:600, letterSpacing:'.05em', padding:'4px 0 8px' }}>TOP 5</div>
          ${tracks.map((t, i) => renderTrackItem(t, i, tracks, '#1e1e1e'))}
        ` : null}

        <!-- albums -->
        ${albums.length ? html`
          <div style=${{ color:'#aaa', fontSize:'11px', fontWeight:600, letterSpacing:'.05em', padding:'12px 0 8px' }}>ALBUM</div>
          <div style=${{ display:'flex', flexDirection:'column', gap:'6px' }}>
            ${albums.map(a => renderRow(a, albums, 0))}
          </div>
        ` : null}

        <!-- related artists -->
        ${related.length ? html`
          <div style=${{ color:'#aaa', fontSize:'11px', fontWeight:600, letterSpacing:'.05em', padding:'12px 0 8px' }}>RELATED ARTISTS</div>
          ${related.map(a => renderRelatedArtistRow(a))}
        ` : null}
      </div>
    `;
  }

  // ─── render ──────────────────────────────────────────────────────────────

  return html`
    <div class="tunein-browser deezer-browser" style=${{ padding:'16px' }}>

      <!-- toolbar -->
      <div style=${{ display:'flex', gap:'8px', marginBottom:'16px' }}>
        <select style=${{ padding:'0 8px', height:'36px', borderRadius:'4px', background:'#333', color:'#fff', border:'none' }}
                value=${searchType} onChange=${(e) => setSearchType(e.target.value)}>
          <option value="album">Album</option>
          <option value="artist">Artist</option>
          <option value="track">Track</option>
        </select>
        <input class="tunein-search-input"
          style=${{ flex:1, padding:'0 12px', height:'36px', borderRadius:'4px', background:'#222', color:'#fff', border:'1px solid #444' }}
          placeholder="Search Deezer..." value=${query}
          onInput=${(e) => setQuery(e.target.value)}
          onKeyDown=${(e) => e.key === 'Enter' && search(query, searchType)} />
        <button style=${{ ...S.pillBtn, background:'#007aff', height:'36px', padding:'0 16px' }}
                onClick=${() => search(query, searchType)}>Search</button>
        <button style=${{ ...S.pillBtn, background:'#444', height:'36px', padding:'0 16px' }}
                onClick=${() => { setQuery(''); setStatus(''); setSections([]); setExpanded({});
                                  setArtistPage(null); setArtistHistory([]); }}>Clear</button>
      </div>

      <div style=${{ color:'#aaa', fontSize:'13px', marginBottom:'8px', minHeight:'20px' }}>${status || ''}</div>
      ${loading ? html`<div class="loading-bar" style=${{ height:'3px', background:'#007aff', width:'100%', marginBottom:'12px' }}></div>` : null}

      <!-- queue panel -->
      <div style=${{ background:'#1b1b1b', border:'1px solid #2a2a2a', borderRadius:'8px', padding:'12px', marginBottom:'20px' }}>
        <div style=${{ display:'flex', justifyContent:'space-between', alignItems:'center', marginBottom: queueOpen ? '10px' : '0' }}>
          <span style=${{ color:'#fff', fontWeight:600, fontSize:'14px', cursor:'pointer', userSelect:'none' }}
                onClick=${() => setQueueOpen(o => !o)}>
            ${queueOpen ? '▾' : '▸'}
            ${' '}${queue.playing ? '▶ In queue' : queue.paused ? '⏸ Paused' : 'Queue'}
            ${queue.upcoming.length ? html` <span style=${{ color:'#666', fontWeight:400 }}>· ${queue.upcoming.length} waiting</span>` : null}
            ${!queueOpen && queue.current ? html` <span style=${{ color:'#34c759', fontWeight:400, fontSize:'12px' }}> — ${queue.current.title}</span>` : null}
          </span>
          <div style=${{ display:'flex', gap:'6px' }}>
            <button style=${{ ...S.pillBtn, background: queue.paused ? '#34c759' : '#3a3a3a',
                              opacity: queue.paused ? 1 : 0.35, cursor: queue.paused ? 'pointer' : 'default' }}
                    disabled=${!queue.paused} onClick=${playQueue} title="Resume">▶ Play</button>
            <button style=${{ ...S.pillBtn, background: queue.playing ? '#e05252' : '#3a3a3a',
                              opacity: queue.playing ? 1 : 0.35, cursor: queue.playing ? 'pointer' : 'default' }}
                    disabled=${!queue.playing} onClick=${stopQueue} title="Stop">■ Stop</button>
            <button style=${{ ...S.pillBtn, background: (queue.playing && queue.upcoming.length > 0) ? '#f0a030' : '#3a3a3a',
                              opacity: (queue.playing && queue.upcoming.length > 0) ? 1 : 0.35,
                              cursor: (queue.playing && queue.upcoming.length > 0) ? 'pointer' : 'default' }}
                    disabled=${!(queue.playing && queue.upcoming.length > 0)} onClick=${skipTrack}>⏭ Next</button>
            <button style=${{ ...S.pillBtn, background: (queue.upcoming.length > 0 || queue.paused) ? '#555' : '#3a3a3a',
                              opacity: (queue.upcoming.length > 0 || queue.paused) ? 1 : 0.35,
                              cursor: (queue.upcoming.length > 0 || queue.paused) ? 'pointer' : 'default' }}
                    disabled=${!(queue.upcoming.length > 0 || queue.paused)} onClick=${clearQueue}>✕ Clear</button>
          </div>
        </div>

        ${queueOpen ? html`
          ${queue.current ? html`
            <div style=${{ display:'flex', alignItems:'center', gap:'10px', padding:'8px 10px', background:'#262626',
                           borderRadius:'6px', marginBottom: queue.upcoming.length ? '8px' : '0', border:'1px solid #3a3a3a' }}>
              ${queue.current.cover_url ? html`<img src=${queue.current.cover_url} style=${{ width:'40px', height:'40px', borderRadius:'4px', objectFit:'cover' }} />` : null}
              <div style=${{ flex:1, minWidth:0 }}>
                <div style=${{ color:'#34c759', fontSize:'11px', fontWeight:600, marginBottom:'2px' }}>▶ NOW PLAYING</div>
                <div style=${{ color:'#fff', fontSize:'14px', fontWeight:500, overflowWrap:'anywhere', wordBreak:'break-word' }}>${queue.current.title}</div>
                <div style=${{ color:'#888', fontSize:'12px' }}>${queue.current.artist}</div>
              </div>
            </div>
          ` : !queue.playing && !queue.paused ? html`
            <div style=${{ color:'#555', fontSize:'13px' }}>No tracks in queue — use + from results.</div>
          ` : null}

          ${queue.upcoming.map((t, i) => html`
            <div key=${`upc-${i}-${t.id}`} style=${{ display:'flex', alignItems:'center', gap:'10px', padding:'6px 10px',
                                                      background: i % 2 === 0 ? '#1e1e1e' : '#222', borderRadius:'4px', marginBottom:'4px' }}>
              <span style=${{ color:'#555', fontSize:'12px', width:'18px', textAlign:'right', flexShrink:0 }}>${i + 1}</span>
              ${t.cover_url ? html`<img src=${t.cover_url} style=${{ width:'32px', height:'32px', borderRadius:'3px', objectFit:'cover' }} />` : null}
              <div style=${{ flex:1, minWidth:0 }}>
                <div style=${{ color:'#ddd', fontSize:'13px', overflowWrap:'anywhere', wordBreak:'break-word' }}>${t.title}</div>
                <div style=${{ color:'#666', fontSize:'11px' }}>${t.artist}</div>
              </div>
              <button style=${{ ...S.pillBtn, background:'transparent', color:'#666', padding:'2px 6px', fontSize:'16px' }}
                      onClick=${() => removeUpcoming(i)}>×</button>
            </div>
          `)}
        ` : null}
      </div>

      <!-- device picker -->
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

      <!-- artist page OR search results -->
      ${artistPage
        ? renderArtistPage()
        : sections.map(section => html`
            <div key=${section.name} style=${{ marginBottom:'24px' }}>
              <h3 style=${{ color:'#fff', borderBottom:'1px solid #333', paddingBottom:'8px', margin:'0 0 10px 0', fontSize:'15px' }}>${section.name}</h3>
              <div style=${{ display:'flex', flexDirection:'column', gap:'6px' }}>
                ${section.items.map(item => renderRow(item, section.items, 0))}
              </div>
            </div>
          `)
      }
    </div>
  `;
}
