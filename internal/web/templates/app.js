// templates/app.js

const API_ROOT = window.API_ROOT;

document.addEventListener('DOMContentLoaded', function() {
    const checkboxes = document.querySelectorAll('.source-checkbox');
    
    const btnAll = document.getElementById('btn-all');
    if(btnAll) {
        btnAll.onclick = () => { checkboxes.forEach(cb => { if (!cb.disabled) cb.checked = true; }); };
    }
    const btnNone = document.getElementById('btn-none');
    if(btnNone) {
        btnNone.onclick = () => { checkboxes.forEach(cb => { if (!cb.disabled) cb.checked = false; }); };
    }

    const initialTypeEl = document.querySelector('input[name="type"]:checked');
    if (initialTypeEl) {
        toggleSearchType(initialTypeEl.value);
    }

    const cards = document.querySelectorAll('.song-card');
    cards.forEach((card, index) => {
        setTimeout(() => inspectSong(card), index * 100);
    });

    cards.forEach(card => {
        const coverBtn = card.querySelector('.btn-cover');
        if (!coverBtn) return;
        const cover = card.dataset.cover || '';
        coverBtn.style.display = cover ? '' : 'none';
    });

    cards.forEach(card => {
        const coverWrap = card.querySelector('.cover-wrapper');
        if (!coverWrap) return;
        
        coverWrap.style.cursor = 'pointer';
        coverWrap.title = '点击生成视频';
        
        coverWrap.onclick = (e) => {
            e.stopPropagation();
            if (window.VideoGen) {
                const img = coverWrap.querySelector('img');
                const currentCover = img ? img.src : (card.dataset.cover || '');

                window.VideoGen.open({
                    id: card.dataset.id,
                    source: card.dataset.source,
                    name: card.dataset.name,
                    artist: card.dataset.artist,
                    cover: currentCover,
                    duration: parseInt(card.dataset.duration) || 0
                });
            } else {
                console.error("VideoGen library not loaded.");
                alert("视频生成组件加载失败，请刷新页面重试");
            }
        };
    });

    syncAllPlayButtons();
});

function toggleSearchType(type) {
    const checkboxes = document.querySelectorAll('.source-checkbox');
    checkboxes.forEach(cb => {
        const isSupported = cb.dataset.supported === "true"; 
        if (type === 'playlist') {
            if (!isSupported) {
                cb.disabled = true;
                cb.checked = false;
            } else {
                cb.disabled = false;
            }
        } else {
            cb.disabled = false;
        }
    });
}

function goToRecommend() {
    const supported = ['netease', 'qq', 'kugou', 'kuwo'];
    const selected = [];
    document.querySelectorAll('.source-checkbox:checked').forEach(cb => {
        if (supported.includes(cb.value)) {
            selected.push(cb.value);
        }
    });
    
    if (selected.length === 0) {
        window.location.href = API_ROOT + '/recommend?sources=' + supported.join('&sources=');
    } else {
        window.location.href = API_ROOT + '/recommend?sources=' + selected.join('&sources=');
    }
}

function inspectSong(card) {
    const id = card.dataset.id;
    const source = card.dataset.source;
    const duration = card.dataset.duration;

    fetch(`${API_ROOT}/inspect?id=${encodeURIComponent(id)}&source=${source}&duration=${duration}`)
        .then(r => r.json())
        .then(data => {
            const sizeTag = document.getElementById(`size-${id}`);
            const bitrateTag = document.getElementById(`bitrate-${id}`);

            if (data.valid) {
                if (sizeTag) {
                    sizeTag.textContent = data.size;
                    sizeTag.className = "tag tag-success"; 
                }
                if (bitrateTag) {
                    bitrateTag.textContent = data.bitrate;
                    bitrateTag.className = "tag";
                }
            } else {
                if (sizeTag) {
                    sizeTag.textContent = "无效";
                    sizeTag.className = "tag tag-fail";
                }
                if (bitrateTag) {
                    bitrateTag.textContent = "-";
                    bitrateTag.className = "tag";
                }
            }
        })
        .catch(() => {
            const el = document.getElementById(`size-${id}`);
            if(el) el.textContent = "检测失败";
        });
}

function openCookieModal() {
    document.getElementById('cookieModal').style.display = 'flex';
    fetch(API_ROOT + '/cookies').then(r => r.json()).then(data => {
        for (const [k, v] of Object.entries(data)) {
            const el = document.getElementById(`cookie-${k}`);
            if(el) el.value = v;
        }
    });
}

function saveCookies() {
    const data = {};
    document.querySelectorAll('input[id^="cookie-"]').forEach(input => {
        if (input.value) data[input.id.replace('cookie-', '')] = input.value;
    });
    fetch(API_ROOT + '/cookies', {
        method: 'POST', 
        body: JSON.stringify(data)
    }).then(() => {
        alert('保存成功！');
        document.getElementById('cookieModal').style.display = 'none';
    });
}

window.addEventListener('scroll', () => {
    const btn = document.getElementById('back-to-top');
    if(!btn) return;
    if (window.scrollY > 300) {
        btn.classList.add('show');
    } else {
        btn.classList.remove('show');
    }
});

function scrollToTop() {
    window.scrollTo({ top: 0, behavior: 'smooth' });
}

// APlayer Config
const ap = new APlayer({
    container: document.getElementById('aplayer'),
    fixed: true, 
    autoplay: false, 
    theme: '#10b981',
    loop: 'all', 
    order: 'list', 
    preload: 'auto', 
    volume: 0.7, 
    listFolded: false, 
    lrcType: 3, 
    audio: []
});

window.ap = ap; 
let currentPlayingId = null;
window.currentPlayingId = null; 

setTimeout(() => {
    const apPic = document.querySelector('.aplayer-pic');
    if (apPic) {
        apPic.style.cursor = 'pointer';
        apPic.title = '点击打开详情/生成视频';
        
        apPic.addEventListener('click', (e) => {
            if (e.target.closest('.aplayer-button') || e.target.closest('.aplayer-play')) {
                return;
            }
            e.stopPropagation();
            e.preventDefault();
            
            const idx = ap.list.index;
            const audio = ap.list.audios[idx];
            
            if (audio && audio.custom_id && window.VideoGen) {
                window.VideoGen.open({
                    id: audio.custom_id,
                    source: audio.source || 'netease',
                    name: audio.name,
                    artist: audio.artist,
                    cover: audio.cover,
                    duration: 0 
                });
            }
        }, true);
    }
}, 800); 

ap.on('listswitch', (e) => {
    const index = e.index;
    const newAudio = ap.list.audios[index];
    if (newAudio && newAudio.custom_id) {
        currentPlayingId = newAudio.custom_id;
        window.currentPlayingId = currentPlayingId; 
        highlightCard(currentPlayingId);
        syncAllPlayButtons();

        const vgModal = document.getElementById("vg-modal");
        if (vgModal && vgModal.classList.contains("active") && window.VideoGen) {
            if (!window.VideoGen.data || window.VideoGen.data.id !== currentPlayingId) {
                window.VideoGen.open({
                    id: newAudio.custom_id,
                    source: newAudio.source || 'netease',
                    name: newAudio.name,
                    artist: newAudio.artist,
                    cover: newAudio.cover,
                    duration: 0
                });
            }
        }
    }
});

ap.on('play', () => {
    const idx = ap?.list?.index;
    const audio = (typeof idx === 'number') ? ap.list.audios[idx] : null;
    if (audio && audio.custom_id) {
        currentPlayingId = audio.custom_id;
        window.currentPlayingId = currentPlayingId; 
        highlightCard(currentPlayingId);
    }
    syncAllPlayButtons();
    
    if (window.VideoGen && window.VideoGen.updatePlayBtnState) {
        window.VideoGen.updatePlayBtnState(true);
    }
});

ap.on('pause', () => {
    syncAllPlayButtons();
    if (window.VideoGen && window.VideoGen.updatePlayBtnState) {
        window.VideoGen.updatePlayBtnState(false);
    }
});

ap.on('ended', () => {
    currentPlayingId = null;
    window.currentPlayingId = null; 
    highlightCard(null);
    syncAllPlayButtons();
});

function highlightCard(targetId) {
    document.querySelectorAll('.song-card').forEach(c => c.classList.remove('playing-active'));
    if(!targetId) return;
    const target = document.querySelector(`.song-card[data-id="${targetId}"]`);
    if (target) {
        target.classList.add('playing-active');
    }
}

function setPlayButtonState(card, isPlaying) {
    if (!card) return;
    const btn = card.querySelector('.btn-play');
    if(!btn) return;
    const icon = btn.querySelector('i');
    if (!icon) return;

    icon.classList.remove('fa-play', 'fa-stop');
    icon.classList.add(isPlaying ? 'fa-stop' : 'fa-play');
    btn.title = isPlaying ? '停止' : '播放';
}

function syncAllPlayButtons() {
    const isActuallyPlaying = ap?.audio && !ap.audio.paused;
    document.querySelectorAll('.song-card').forEach(card => {
        const id = card.dataset.id;
        const active = isActuallyPlaying && currentPlayingId && id === currentPlayingId;
        setPlayButtonState(card, active);
    });
}

function formatDuration(seconds) {
    const s = Number(seconds || 0);
    if (!s || s <= 0) return '-';
    const min = Math.floor(s / 60);
    const sec = Math.floor(s % 60);
    return `${String(min).padStart(2, '0')}:${String(sec).padStart(2, '0')}`;
}

function updateCardWithSong(card, song) {
    const oldId = card.dataset.id; 

    card.dataset.id = song.id;
    card.dataset.source = song.source;
    card.dataset.duration = song.duration || 0;
    card.dataset.name = song.name || card.dataset.name;
    card.dataset.artist = song.artist || card.dataset.artist;
    card.dataset.cover = song.cover || '';

    const titleEl = card.querySelector('.song-info h3');
    if (titleEl) {
        if (song.link) {
            titleEl.innerHTML = `<a href="${song.link}" target="_blank" class="song-title-link" title="打开原始链接">${song.name || ''}</a>`;
        } else {
            titleEl.textContent = song.name || '';
        }
    }

    const artistLine = card.querySelector('.artist-line');
    if (artistLine) {
        const albumText = song.album ? ` &nbsp;•&nbsp; ${song.album}` : '';
        artistLine.innerHTML = `<i class="fa-regular fa-user" style="font-size:11px;"></i> ${song.artist || ''}${albumText}`;
    }

    const sourceTag = card.querySelector('.tag-src');
    if (sourceTag) sourceTag.textContent = song.source;

    const tags = card.querySelectorAll('.tags .tag');
    if (tags && tags.length >= 2) {
        tags[1].textContent = formatDuration(song.duration);
    }

    const coverWrap = card.querySelector('.cover-wrapper');
    if (coverWrap) {
        let imgEl = coverWrap.querySelector('img');
        if (!imgEl) {
            imgEl = document.createElement('img');
            coverWrap.innerHTML = '';
            coverWrap.appendChild(imgEl);
        }
        imgEl.src = song.cover || 'https://via.placeholder.com/150?text=Music';
        imgEl.alt = song.name || '';
        
        coverWrap.onclick = (e) => {
            e.stopPropagation();
            if (window.VideoGen) {
                window.VideoGen.open({
                    id: card.dataset.id,
                    source: card.dataset.source,
                    name: card.dataset.name,
                    artist: card.dataset.artist,
                    cover: imgEl.src,
                    duration: parseInt(card.dataset.duration) || 0
                });
            }
        };
    }

    const dl = card.querySelector('.btn-download');
    if (dl) {
        dl.href = `${API_ROOT}/download?id=${encodeURIComponent(song.id)}&source=${song.source}&name=${encodeURIComponent(song.name)}&artist=${encodeURIComponent(song.artist)}`;
        dl.id = `dl-${song.id}`;
    }

    const lrc = card.querySelector('.btn-lyric');
    if (lrc) {
        lrc.href = `${API_ROOT}/download_lrc?id=${encodeURIComponent(song.id)}&source=${song.source}&name=${encodeURIComponent(song.name)}&artist=${encodeURIComponent(song.artist)}`;
        lrc.id = `lrc-${song.id}`;
    }

    const coverBtn = card.querySelector('.btn-cover');
    if (coverBtn) {
        if (song.cover) {
            coverBtn.style.display = '';
            coverBtn.href = `${API_ROOT}/download_cover?url=${encodeURIComponent(song.cover)}&name=${encodeURIComponent(song.name)}&artist=${encodeURIComponent(song.artist)}`;
        } else {
            coverBtn.style.display = 'none';
        }
    }

    const sizeTag = card.querySelector('[id^="size-"]');
    if (sizeTag) {
        sizeTag.id = `size-${song.id}`;
        sizeTag.className = 'tag tag-loading';
        sizeTag.innerHTML = '<i class="fa fa-spinner fa-spin"></i>';
    }
    const bitrateTag = card.querySelector('[id^="bitrate-"]');
    if (bitrateTag) {
        bitrateTag.id = `bitrate-${song.id}`;
        bitrateTag.className = 'tag tag-loading';
        bitrateTag.innerHTML = '<i class="fa fa-circle-notch fa-spin"></i>';
    }

    if (currentPlayingId === oldId) {
        currentPlayingId = song.id;
    }
    syncAllPlayButtons();
    inspectSong(card);
    syncSongToAPlayer(oldId, song);
}

function syncSongToAPlayer(oldId, newSong) {
    if (!ap || !ap.list || !ap.list.audios) return;
    const index = ap.list.audios.findIndex(a => a.custom_id === oldId);
    if (index !== -1) {
        const audio = ap.list.audios[index];
        audio.name = newSong.name;
        audio.artist = newSong.artist;
        audio.cover = newSong.cover;
        audio.url = `${API_ROOT}/download?id=${encodeURIComponent(newSong.id)}&source=${newSong.source}&name=${encodeURIComponent(newSong.name)}&artist=${encodeURIComponent(newSong.artist)}`;
        audio.lrc = `${API_ROOT}/lyric?id=${encodeURIComponent(newSong.id)}&source=${newSong.source}`;
        audio.custom_id = newSong.id; 
        audio.source = newSong.source; 
        
        if (ap.list.index === index) {
            ap.list.switch(index); 
            if (ap.audio.paused) {
                ap.play();
            }
        }
    }
}

function switchSource(btn) {
    const card = btn.closest('.song-card');
    if (!card) return;

    const ds = card.dataset;
    const name = ds.name || '';
    const artist = ds.artist || '';
    const source = ds.source || '';
    if (!name || !source) return;

    btn.disabled = true;
    btn.style.opacity = '0.6';

    const duration = ds.duration || '';
    const url = `${API_ROOT}/switch_source?name=${encodeURIComponent(name)}&artist=${encodeURIComponent(artist)}&source=${encodeURIComponent(source)}&duration=${encodeURIComponent(duration)}`;
    fetch(url)
        .then(r => r.ok ? r.json() : Promise.reject())
        .then(song => {
            updateCardWithSong(card, song);
        })
        .catch(() => {
            alert('换源失败，请稍后重试');
        })
        .finally(() => {
            btn.disabled = false;
            btn.style.opacity = '1';
        });
}

function playAllAndJumpTo(btn) {
    const currentCard = btn.closest('.song-card');
    const allCards = Array.from(document.querySelectorAll('.song-card'));
    const clickedIndex = allCards.indexOf(currentCard);

    if (clickedIndex === -1) return;

    const clickedId = currentCard.dataset.id;
    const isActuallyPlaying = ap?.audio && !ap.audio.paused;

    if (currentPlayingId && currentPlayingId === clickedId && isActuallyPlaying) {
        ap.pause();
        try { ap.seek(0); } catch (e) {}
        currentPlayingId = null;
        highlightCard(null);
        syncAllPlayButtons();
        return;
    }

    ap.list.clear();
    const playlist = [];

    allCards.forEach(card => {
        const ds = card.dataset;
        let coverUrl = '';
        const imgEl = card.querySelector('.cover-wrapper img');
        if (imgEl) coverUrl = imgEl.src;

        playlist.push({
            name: ds.name,
            artist: ds.artist,
            url: `${API_ROOT}/download?id=${encodeURIComponent(ds.id)}&source=${ds.source}&name=${encodeURIComponent(ds.name)}&artist=${encodeURIComponent(ds.artist)}`,
            cover: coverUrl,
            lrc: `${API_ROOT}/lyric?id=${encodeURIComponent(ds.id)}&source=${ds.source}`,
            theme: '#10b981',
            custom_id: ds.id,
            source: ds.source
        });
    });

    ap.list.add(playlist);
    ap.list.switch(clickedIndex);
    ap.play();

    currentPlayingId = clickedId;
    highlightCard(currentPlayingId);
    syncAllPlayButtons();
}

window.playAllAndJumpToId = function(songId) {
    const targetCard = document.querySelector(`.song-card[data-id="${songId}"]`);
    if (targetCard) {
        const btn = targetCard.querySelector('.btn-play');
        if (btn) {
            playAllAndJumpTo(btn);
        }
    }
};

let isBatchMode = false;

function toggleBatchMode() {
    isBatchMode = !isBatchMode;
    document.body.classList.toggle('batch-mode', isBatchMode);
    const btn = document.getElementById('btn-batch-toggle');
    const toolbar = document.getElementById('batch-toolbar');
    
    if(!btn || !toolbar) return;

    if (isBatchMode) {
        btn.innerHTML = '<i class="fa-solid fa-xmark"></i> 退出批量';
        btn.style.color = 'var(--error-color)';
        toolbar.classList.add('active'); 
    } else {
        btn.innerHTML = '<i class="fa-solid fa-list-check"></i> 批量操作';
        btn.style.color = 'var(--text-sub)';
        toolbar.classList.remove('active');
        document.querySelectorAll('.song-checkbox').forEach(cb => cb.checked = false);
        updateBatchToolbar();
    }
}

function updateBatchToolbar() {
    const checkedBoxes = document.querySelectorAll('.song-checkbox:checked');
    const count = checkedBoxes.length;
    const selectAllCb = document.getElementById('select-all-checkbox');
    const batchSwitch = document.getElementById('btn-batch-switch');
    const batchDl = document.getElementById('btn-batch-dl');
    
    if(document.getElementById('selected-count')) {
        document.getElementById('selected-count').textContent = count;
    }
    
    const allBoxes = document.querySelectorAll('.song-checkbox');
    if (allBoxes.length > 0 && selectAllCb) {
        selectAllCb.checked = (allBoxes.length === count);
    }

    if (count > 0) {
        if(batchSwitch) batchSwitch.disabled = false;
        if(batchDl) batchDl.disabled = false;
    } else {
        if(batchSwitch) batchSwitch.disabled = true;
        if(batchDl) batchDl.disabled = true;
    }
    
    document.querySelectorAll('.song-card').forEach(card => card.classList.remove('selected'));
    checkedBoxes.forEach(cb => {
        cb.closest('.song-card').classList.add('selected');
    });
}

function toggleSelectAll(mainCb) {
    const checkboxes = document.querySelectorAll('.song-checkbox');
    checkboxes.forEach(cb => cb.checked = mainCb.checked);
    updateBatchToolbar();
}

function selectInvalidSongs() {
    const invalidTags = document.querySelectorAll('.tag-fail');
    if (invalidTags.length === 0) {
        alert('当前列表中没有检测到无效歌曲');
        return;
    }
    
    let count = 0;
    invalidTags.forEach(tag => {
        const card = tag.closest('.song-card');
        if (card) {
            const cb = card.querySelector('.song-checkbox');
            if (cb && !cb.checked) {
                cb.checked = true;
                count++;
            }
        }
    });
    
    if (count === 0) {
        alert('无效歌曲已全部选中');
    }
    updateBatchToolbar();
}

function getSelectedSongs() {
    const checkedBoxes = document.querySelectorAll('.song-checkbox:checked');
    const songs = [];
    checkedBoxes.forEach(cb => {
        const card = cb.closest('.song-card');
        if (card) {
            const ds = card.dataset;
            let coverUrl = '';
            const imgEl = card.querySelector('.cover-wrapper img');
            if (imgEl) coverUrl = imgEl.src;
            
            songs.push({
                id: ds.id,
                source: ds.source,
                name: ds.name,
                artist: ds.artist,
                url: `${API_ROOT}/download?id=${encodeURIComponent(ds.id)}&source=${ds.source}&name=${encodeURIComponent(ds.name)}&artist=${encodeURIComponent(ds.artist)}`,
                cover: coverUrl,
                lrc: `${API_ROOT}/lyric?id=${encodeURIComponent(ds.id)}&source=${ds.source}`,
                theme: '#10b981',
                custom_id: ds.id
            });
        }
    });
    return songs;
}

function batchDownload() {
    const songs = getSelectedSongs();
    if (songs.length === 0) return;

    if (!confirm(`准备下载 ${songs.length} 首歌曲。\n注意：浏览器可能会拦截多个弹窗，请务必允许本站点的弹窗！`)) {
        return;
    }

    songs.forEach((s, index) => {
        setTimeout(() => {
            const link = document.createElement('a');
            link.href = s.url;
            link.download = ''; 
            link.target = '_blank';
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
        }, index * 800); 
    });
}

function batchSwitchSource() {
    const checkedBoxes = document.querySelectorAll('.song-checkbox:checked');
    if (checkedBoxes.length === 0) return;

    if (!confirm(`准备对 ${checkedBoxes.length} 首歌曲进行自动换源。\n这可能需要一些时间，请耐心等待。`)) {
        return;
    }

    checkedBoxes.forEach((cb, index) => {
        const card = cb.closest('.song-card');
        if (card) {
            const switchBtn = card.querySelector('.btn-switch');
            if (switchBtn) {
                setTimeout(() => {
                    switchSource(switchBtn);
                }, index * 1000); 
            }
        }
    });
}


// ==========================================
// 收藏夹系统前端逻辑
// ==========================================

let pendingFavSong = null; // 暂存当前准备加入收藏夹的歌曲数据
let currentViewColId = null; // 当前正在查看详情的收藏夹 ID
let currentViewColSongs = []; // 暂存当前详情界面的所有歌曲(用来全量播放)

// 1. 打开主收藏夹管理面板
function openCollectionManager() {
    document.getElementById('collectionManagerModal').style.display = 'flex';
    fetchAndRenderCollections('colList', true);
}

// 2. 加载收藏夹列表
// withActions = true 时显示 删除/查看 按钮 (主面板)
// withActions = false 时显示 选中 加入 功能 (添加歌曲面板)
function fetchAndRenderCollections(containerId, withActions) {
    const container = document.getElementById(containerId);
    container.innerHTML = '<div style="text-align: center; color: #a0aec0; padding: 20px;">加载中...</div>';
    
    fetch(API_ROOT + '/collections')
        .then(r => r.json())
        .then(data => {
            if (!data || data.length === 0) {
                container.innerHTML = '<div style="text-align: center; color: #a0aec0; padding: 20px;">暂无收藏夹</div>';
                return;
            }
            container.innerHTML = '';
            data.forEach(col => {
                const item = document.createElement('div');
                item.className = 'collection-item';
                
                let actionHtml = '';
                if (withActions) {
                    actionHtml = `
                        <div class="collection-actions">
                            <button class="col-btn" onclick="openCollectionDetail('${col.id}', '${col.name}')">查看</button>
                            <button class="col-btn del" onclick="deleteCollection('${col.id}')">删除</button>
                        </div>
                    `;
                    item.innerHTML = `
                        <div class="collection-info" onclick="openCollectionDetail('${col.id}', '${col.name}')">
                            <div class="collection-name">${col.name}</div>
                            <div class="collection-desc">${col.description || '无描述'}</div>
                        </div>
                        ${actionHtml}
                    `;
                } else {
                    item.style.cursor = 'pointer';
                    item.innerHTML = `
                        <div class="collection-info">
                            <div class="collection-name">${col.name}</div>
                            <div class="collection-desc">${col.description || ''}</div>
                        </div>
                        <i class="fa-solid fa-plus" style="color: #10b981;"></i>
                    `;
                    item.onclick = () => { addSongToCollection(col.id); };
                }
                container.appendChild(item);
            });
        })
        .catch(() => {
            container.innerHTML = '<div style="text-align: center; color: #e53e3e; padding: 20px;">加载失败</div>';
        });
}

// 3. 创建新收藏夹
function createCollection() {
    const nameInput = document.getElementById('newColName');
    const name = nameInput.value.trim();
    if (!name) return alert('请输入收藏夹名称');
    
    fetch(API_ROOT + '/collections', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: name, description: '' })
    }).then(r => r.json()).then(res => {
        if (res.error) return alert(res.error);
        nameInput.value = '';
        fetchAndRenderCollections('colList', true);
    });
}

// 4. 删除收藏夹
function deleteCollection(id) {
    if (!confirm("确定要删除此收藏夹及其包含的所有歌曲记录吗？")) return;
    fetch(`${API_ROOT}/collections/${id}`, { method: 'DELETE' })
        .then(() => fetchAndRenderCollections('colList', true));
}

// 5. 点击列表内单曲的爱心 -> 弹出选择收藏夹面板
function openAddToCollectionModal(btn) {
    const card = btn.closest('.song-card');
    if (!card) return;
    
    let coverUrl = '';
    const imgEl = card.querySelector('.cover-wrapper img');
    if (imgEl) coverUrl = imgEl.src;

    pendingFavSong = {
        id: card.dataset.id,
        source: card.dataset.source,
        name: card.dataset.name,
        artist: card.dataset.artist,
        duration: parseInt(card.dataset.duration) || 0,
        cover: coverUrl,
        extra: { saved_from: "web_ui" }
    };
    
    document.getElementById('addToCollectionModal').style.display = 'flex';
    fetchAndRenderCollections('addColList', false);
}

// 6. 确认添加到目标收藏夹
function addSongToCollection(colId) {
    if (!pendingFavSong) return;
    
    fetch(`${API_ROOT}/collections/${colId}/songs`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(pendingFavSong)
    }).then(r => r.json()).then(res => {
        if (res.error) {
            alert(res.error);
        } else {
            alert('已成功添加到收藏夹！');
            document.getElementById('addToCollectionModal').style.display = 'none';
        }
    });
}

// 7. 打开并查看收藏夹详情 (歌曲列表)
function openCollectionDetail(colId, colName) {
    currentViewColId = colId;
    document.getElementById('colDetailTitle').textContent = colName;
    document.getElementById('collectionManagerModal').style.display = 'none'; // hide upper modal
    document.getElementById('collectionDetailModal').style.display = 'flex';
    
    const container = document.getElementById('colSongsList');
    container.innerHTML = '<div style="text-align: center; color: #a0aec0; padding: 20px;">加载中...</div>';
    
    fetch(`${API_ROOT}/collections/${colId}/songs`)
        .then(r => r.json())
        .then(songs => {
            currentViewColSongs = songs || [];
            if (!songs || songs.length === 0) {
                container.innerHTML = '<div style="text-align: center; color: #a0aec0; padding: 20px;">该收藏夹还是空的</div>';
                return;
            }
            container.innerHTML = '';
            songs.forEach((song, idx) => {
                const item = document.createElement('div');
                item.className = 'collection-item';
                item.style.background = '#fff';
                const fDur = formatDuration(song.duration);
                item.innerHTML = `
                    <div class="cover-wrapper" style="width: 45px; height: 45px; margin-right: 12px; border-radius: 6px;">
                        <img src="${song.cover || 'https://via.placeholder.com/150'}" loading="lazy">
                    </div>
                    <div class="collection-info">
                        <div class="collection-name" style="font-size:14px;">${song.name}</div>
                        <div class="collection-desc">${song.artist} <span class="tag tag-src" style="margin-left:5px;">${song.source}</span> <span class="tag">${fDur}</span></div>
                    </div>
                    <div class="collection-actions">
                        <button class="col-btn" title="播放此歌" onclick="playSingleFromCollection(${idx})"><i class="fa-solid fa-play"></i></button>
                        <button class="col-btn del" title="移出收藏" onclick="removeSongFromCollection('${colId}', '${song.id}', '${song.source}')"><i class="fa-solid fa-trash"></i></button>
                    </div>
                `;
                container.appendChild(item);
            });
        }).catch(() => {
            container.innerHTML = '<div style="text-align: center; color: #e53e3e; padding: 20px;">加载失败</div>';
        });
}

// 8. 将整张收藏夹推入 APlayer 播放列表
function playCurrentCollection() {
    if (!currentViewColSongs || currentViewColSongs.length === 0) return alert('列表为空');
    
    ap.list.clear();
    const playlist = currentViewColSongs.map(ds => ({
        name: ds.name,
        artist: ds.artist,
        url: `${API_ROOT}/download?id=${encodeURIComponent(ds.id)}&source=${ds.source}&name=${encodeURIComponent(ds.name)}&artist=${encodeURIComponent(ds.artist)}`,
        cover: ds.cover || 'https://via.placeholder.com/150',
        lrc: `${API_ROOT}/lyric?id=${encodeURIComponent(ds.id)}&source=${ds.source}`,
        theme: '#10b981',
        custom_id: ds.id,
        source: ds.source
    }));

    ap.list.add(playlist);
    ap.play();
    document.getElementById('collectionDetailModal').style.display = 'none';
}

// 9. 播放收藏夹里的单曲 (自动把整个收藏夹作为列表，定位到该曲)
function playSingleFromCollection(index) {
    if (!currentViewColSongs || currentViewColSongs.length === 0) return;
    
    ap.list.clear();
    const playlist = currentViewColSongs.map(ds => ({
        name: ds.name,
        artist: ds.artist,
        url: `${API_ROOT}/download?id=${encodeURIComponent(ds.id)}&source=${ds.source}&name=${encodeURIComponent(ds.name)}&artist=${encodeURIComponent(ds.artist)}`,
        cover: ds.cover || 'https://via.placeholder.com/150',
        lrc: `${API_ROOT}/lyric?id=${encodeURIComponent(ds.id)}&source=${ds.source}`,
        theme: '#10b981',
        custom_id: ds.id,
        source: ds.source
    }));

    ap.list.add(playlist);
    ap.list.switch(index);
    ap.play();
}

// 10. 从收藏夹移除歌曲
function removeSongFromCollection(colId, songId, source) {
    if (!confirm('确定将此歌曲移出收藏吗？')) return;
    fetch(`${API_ROOT}/collections/${colId}/songs?id=${encodeURIComponent(songId)}&source=${encodeURIComponent(source)}`, { method: 'DELETE' })
        .then(() => openCollectionDetail(colId, document.getElementById('colDetailTitle').textContent));
}