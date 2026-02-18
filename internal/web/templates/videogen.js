(function () {
    // =================================================================
    // 共享核心算法：确保网页实时播放与离线视频渲染的效果 100% 一致
    // =================================================================
    
    const FFT = {
        windowed: null, mags: null, previousMags: null,
        reset: function() { this.previousMags = null; },
        fft: function(data) {
            const n = data.length;
            if (n <= 1) return data;
            const half = n / 2, even = new Float32Array(half), odd = new Float32Array(half);
            for (let i = 0; i < half; i++) { even[i] = data[2 * i]; odd[i] = data[2 * i + 1]; }
            const q = this.fft(even), r = this.fft(odd), output = new Float32Array(n); 
            for (let k = 0; k < half; k++) { const t = r[k]; output[k] = q[k] + t; output[k + half] = q[k] - t; }
            return output;
        },
        getFrequencyData: function(pcmData, fftSize, smoothing) {
            const half = fftSize / 2;
            if (!this.windowed || this.windowed.length !== fftSize) {
                this.windowed = new Float32Array(fftSize);
                this.mags = new Uint8Array(half);
                this.previousMags = new Float32Array(half);
            }
            for(let i=0; i<fftSize; i++) {
                const val = (i < pcmData.length) ? pcmData[i] : 0;
                this.windowed[i] = val * (0.5 * (1 - Math.cos(2 * Math.PI * i / (fftSize - 1)))); 
            }
            const rawFFT = this.fft(this.windowed);
            for(let i=0; i<half; i++) {
                let mag = Math.abs(rawFFT[i]) / fftSize;
                mag = mag * 2.0; 
                mag = smoothing * this.previousMags[i] + (1 - smoothing) * mag;
                this.previousMags[i] = mag;
                let db = 20 * Math.log10(mag + 1e-6);
                let val = (db + 100) * (255 / 70); 
                if(val < 0) val = 0; if(val > 255) val = 255;
                this.mags[i] = val;
            }
            return this.mags;
        }
    };

    function processVisualizerBars(freqData) {
        const barsCount = 180; 
        const barHeights = [];
        for(let i=0; i<barsCount; i++) {
            const minIdx = 1, maxIdx = freqData.length / 2;
            const logRange = Math.log(maxIdx / minIdx);
            const idx = minIdx * Math.exp(logRange * (i / barsCount));
            const lower = Math.floor(idx), upper = Math.ceil(idx), frac = idx - lower;
            let val = (freqData[lower] || 0) * (1 - frac) + (freqData[upper] || 0) * frac;
            
            const weight = 1 + (i / barsCount) * 1.5;
            val *= weight;
            if (val > 255) val = 255;

            const threshold = 170; 
            let h = 2; 
            if (val > threshold) {
                let active = (val - threshold) / (255 - threshold);
                if (active > 1.0) active = 1.0;
                h += Math.pow(active, 3.0) * 33; 
            }
            if (h > 35) h = 35; 
            barHeights.push(h);
        }
        return { heights: barHeights };
    }

    function drawVisualizerRings(ctx, cx, cy, radius, heights) {
        ctx.save();
        ctx.translate(cx, cy);
        const barsCount = heights.length, barWidth = 1.5, halfWidth = barWidth / 2;
        for (let i = 0; i < barsCount; i++) {
            ctx.save();
            const angle = (Math.PI * 2 / barsCount) * i - Math.PI / 2;
            ctx.rotate(angle);
            const h = heights[i] || 2;
            const hue = (i / barsCount) * 360; 
            ctx.fillStyle = `hsla(${hue}, 100%, 65%, 0.9)`;
            ctx.beginPath();
            if (ctx.roundRect) {
                ctx.roundRect(-halfWidth, -radius - h, barWidth, h, 0.5);
            } else {
                ctx.rect(-halfWidth, -radius - h, barWidth, h);
            }
            ctx.fill();
            ctx.restore(); 
        }
        ctx.restore(); 
    }

    // =================================================================
    // 独立新窗口渲染线程 (Worker 环境)
    // =================================================================
    if (window.isRenderWorker) {
        async function runOfflineRender(data) {
            const apiRoot = data.apiRoot;
            const statusText = document.getElementById("status-text");
            const progressFill = document.getElementById("progress-fill");
            const titleEl = document.getElementById("title");
            const previewCanvas = document.getElementById("preview-canvas");
            
            const setStatus = (title, desc, pct) => {
                if(title) titleEl.textContent = title;
                if(desc) statusText.textContent = desc;
                if(pct !== undefined) progressFill.style.width = pct + "%";
            };

            try {
                setStatus("正在初始化...", "请求云端处理通道", 5);
                const initRes = await fetch(`${apiRoot}/videogen/init`, {
                    method: "POST", headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({ id: data.id, source: data.source }),
                }).then((r) => r.json());
                
                if (initRes.error) throw new Error(initRes.error);
                
                setStatus("下载与解码音频...", "可能需要一些时间，请耐心等待", 15);
                const audioCtx = new (window.AudioContext || window.webkitAudioContext)();
                const audioBuffer = await fetch(initRes.audio_url)
                    .then((r) => r.arrayBuffer())
                    .then((arr) => audioCtx.decodeAudioData(arr));
                    
                setStatus("加载视觉资源...", "准备 1080P 超清渲染画板", 25);
                
                // 1080P 设置
                const logicalW = 1280, logicalH = 720, scaleFactor = 1.5; 
                const width = logicalW * scaleFactor, height = logicalH * scaleFactor;
                
                const canvas = document.createElement("canvas");
                canvas.width = width; canvas.height = height;
                const ctx = canvas.getContext("2d");
                
                previewCanvas.width = width; previewCanvas.height = height;
                const previewCtx = previewCanvas.getContext("2d");

                let bgMedia = null;
                if (data.isVideoBg) {
                    bgMedia = document.createElement("video");
                    bgMedia.src = data.rawCover; bgMedia.muted = true; bgMedia.loop = true;
                    bgMedia.setAttribute('playsinline', ''); 
                    await bgMedia.play(); bgMedia.pause(); 
                } else {
                    bgMedia = new Image(); bgMedia.crossOrigin = "Anonymous";
                    let coverSrc = data.rawCover;
                    if (!data.rawCover.startsWith("data:")) coverSrc = `${apiRoot}/download_cover?url=${encodeURIComponent(data.rawCover)}&name=render&artist=render`;
                    await Promise.race([
                        new Promise(r => { bgMedia.onload = r; bgMedia.onerror = () => { bgMedia.src = "https://via.placeholder.com/600"; setTimeout(r, 1000); }; bgMedia.src = coverSrc; }),
                        new Promise((_, r) => setTimeout(() => r(new Error("资源加载超时")), 15000))
                    ]);
                }
                
                const fps = 30;
                const duration = audioBuffer.duration;
                const totalFrames = Math.floor(duration * fps);
                const rawData = audioBuffer.getChannelData(0);
                const samplesPerFrame = Math.floor(audioBuffer.sampleRate / fps);
                const batchSize = 30; 
                
                FFT.reset();
                setStatus("超清渲染中", "0%", 30);
                
                const uploadBatch = async (frames, startIdx) => {
                    await fetch(`${apiRoot}/videogen/frame`, {
                        method: "POST", headers: { "Content-Type": "application/json" },
                        body: JSON.stringify({ session_id: initRes.session_id, frames: frames, start_idx: startIdx })
                    });
                };
                
                const seekVideo = async (time) => {
                    if (!data.isVideoBg || !bgMedia.duration) return;
                    const tt = time % bgMedia.duration;
                    bgMedia.currentTime = tt;
                    if (Math.abs(bgMedia.currentTime - tt) < 0.1 && bgMedia.readyState >= 3) return;
                    await new Promise(r => {
                        const onSeeked = () => { bgMedia.removeEventListener('seeked', onSeeked); r(); };
                        setTimeout(() => { bgMedia.removeEventListener('seeked', onSeeked); r(); }, 500); 
                        bgMedia.addEventListener('seeked', onSeeked);
                    });
                };
                
                const drawFrame = async (frameIdx) => {
                    const time = frameIdx / fps;
                    if (data.isVideoBg) await seekVideo(time);
          
                    const startSample = frameIdx * samplesPerFrame;
                    const pcmSlice = rawData.slice(startSample, startSample + 256);
                    // 传入 0.4 平滑度参数，确保动画与实时播放同步干脆
                    const freqData = FFT.getFrequencyData(pcmSlice, 256, 0.4);
                    const visResult = processVisualizerBars(freqData);
          
                    ctx.clearRect(0, 0, width, height); 
                    ctx.save();
                    // 放大到 1080P
                    ctx.scale(scaleFactor, scaleFactor);
                    
                    let mw = data.isVideoBg ? bgMedia.videoWidth : bgMedia.width;
                    let mh = data.isVideoBg ? bgMedia.videoHeight : bgMedia.height;
                    if (!mw) mw = logicalW; if (!mh) mh = logicalH; 
          
                    const baseRatio = Math.max(logicalW / mw, logicalH / mh);
                    let imgScale = 1.0;
                    if (!data.isVideoBg) {
                        const cycle = 20, progress = (time % (cycle * 2)) / cycle, ease = progress < 1 ? progress : 2 - progress; 
                        imgScale = 1.0 + (ease * ease * (3 - 2 * ease) * 0.1);
                    }
                    
                    const finalRatio = baseRatio * imgScale;
                    const bgW = mw * finalRatio, bgH = mh * finalRatio;
                    const bgX = (logicalW - bgW) / 2, bgY = (logicalH - bgH) / 2;
                    
                    ctx.drawImage(bgMedia, bgX, bgY, bgW, bgH);
          
                    const cx = 320, cy = logicalH / 2, discRadius = 200, barBaseRadius = discRadius + 2; 
                    drawVisualizerRings(ctx, cx, cy, barBaseRadius, visResult.heights);
        
                    ctx.save();
                    ctx.translate(cx, cy);
                    ctx.beginPath(); ctx.arc(0, 0, discRadius, 0, Math.PI * 2); ctx.fillStyle = "#111"; ctx.fill();
                    ctx.strokeStyle = "rgba(255,255,255,0.1)"; ctx.lineWidth = 4; ctx.stroke();
                    
                    const grad = ctx.createRadialGradient(0,0,discRadius*0.5, 0,0,discRadius);
                    grad.addColorStop(0, '#1a1a1a'); grad.addColorStop(0.5, '#222'); grad.addColorStop(1, '#111');
                    ctx.fillStyle = grad; ctx.fill();
          
                    ctx.save(); ctx.rotate(time * 0.4); ctx.beginPath(); ctx.arc(0, 0, coverRadius = discRadius * 0.65, 0, Math.PI * 2); ctx.clip(); 
                    ctx.drawImage(bgMedia, 0, 0, mw, mh, -coverRadius, -coverRadius, coverRadius * 2, coverRadius * 2); ctx.restore();
                    ctx.restore(); 
          
                    const lx = 700, ly = logicalH / 2;
                    let activeIdx = -1;
                    for (let i = 0; i < data.lyricRaw.length; i++) { if (time >= data.lyricRaw[i].time) activeIdx = i; else break; }
          
                    ctx.textAlign = "left"; ctx.textBaseline = "middle";
                    for (let offset = -4; offset <= 4; offset++) {
                      const idx = activeIdx + offset;
                      if (idx >= 0 && idx < data.lyricRaw.length) {
                        const isCurrent = offset === 0;
                        ctx.font = isCurrent ? "bold 36px sans-serif" : "600 26px sans-serif";
                        ctx.fillStyle = isCurrent ? "#fff" : "rgba(255,255,255,0.85)";
                        ctx.shadowColor = "rgba(0,0,0,0.9)";
                        ctx.shadowBlur = isCurrent ? 6 : 4; 
                        ctx.shadowOffsetX = isCurrent ? 2 : 1; 
                        ctx.shadowOffsetY = isCurrent ? 2 : 1;
                        ctx.fillText(data.lyricRaw[idx].text, lx, ly + offset * 65);
                      }
                    }
                    
                    ctx.font = "bold 26px sans-serif"; ctx.fillStyle = "#fff"; ctx.textAlign = "center";
                    ctx.shadowColor = "rgba(0,0,0,0.9)"; ctx.shadowBlur = 8;
                    ctx.fillText(data.name, cx, logicalH - 50);
                    ctx.font = "18px sans-serif"; ctx.fillStyle = "rgba(255,255,255,0.9)";
                    ctx.fillText(data.artist, cx, logicalH - 20);
                    
                    ctx.restore(); 
                    
                    // 将高清帧绘制到屏幕的小预览窗上
                    previewCtx.clearRect(0,0,width,height);
                    previewCtx.drawImage(canvas, 0, 0);
                };
                
                let frameIdx = 0;
                while (frameIdx < totalFrames) {
                  let framesBuffer = [];
                  const batchStartIdx = frameIdx;
                  for (let i = 0; i < batchSize && frameIdx < totalFrames; i++) {
                    await drawFrame(frameIdx);
                    framesBuffer.push(canvas.toDataURL("image/jpeg", 0.95));
                    frameIdx++;
                  }
                  await uploadBatch(framesBuffer, batchStartIdx);
                  const pct = Math.round((frameIdx / totalFrames) * 100);
                  setStatus("超清帧渲染中...", `已完成 ${pct}%  (${frameIdx}/${totalFrames} 帧)`, 30 + pct * 0.65);
                  await new Promise(r => setTimeout(r, 0));
                }
                
                setStatus("正在合成最终视频...", "合并无损音频与画面帧", 98);
                const finalRes = await fetch(`${apiRoot}/videogen/finish`, {
                  method: "POST", headers: { "Content-Type": "application/json" },
                  body: JSON.stringify({ session_id: initRes.session_id, name: `${data.name} - ${data.artist}` }),
                }).then(r => r.json());
        
                if (finalRes.error) throw new Error(finalRes.error);
                
                document.getElementById('loading-ui').style.display = 'none';
                document.getElementById('success-ui').style.display = 'block';
                document.getElementById("dl-link").href = apiRoot + finalRes.url;
                document.getElementById("dl-link").download = `${data.name}.mp4`;
                
            } catch(e) {
                console.error(e);
                document.getElementById('loading-ui').style.display = 'none';
                document.getElementById('error-ui').style.display = 'block';
                document.getElementById('error-text').textContent = e.message;
            }
        }
        
        runOfflineRender(window.renderData);
        return; // 终止 Worker 中的后续 UI 初始化
    }


    // =================================================================
    // 原主页面 UI 逻辑 (网页播放界面)
    // =================================================================
    window.VideoGen = {
      data: null, customVisual: null, lyricTimes: [], lyricRaw: [], lastActiveIndex: -1,
      audioCtx: null, analyser: null, sourceNode: null, isPlaying: false,
      rtCanvas: null, rtCtx: null, animationId: null, isVideoBg: false,
      resizeObserver: null, isDraggingProgress: false,

      apTimeHandler: null, apPlayHandler: null, apPauseHandler: null, apEndHandler: null,
  
      formatTime: function(s) {
          if (isNaN(s)) return "00:00";
          const m = Math.floor(s / 60), sec = Math.floor(s % 60);
          return `${m < 10 ? '0' : ''}${m}:${sec < 10 ? '0' : ''}${sec}`;
      },

      handleFileSelect: function (input) {
        if (input.files && input.files[0]) {
          const file = input.files[0], reader = new FileReader();
          reader.onload = (e) => { this.customVisual = e.target.result; this.updateVisuals(this.customVisual, file.type.startsWith("video/")); };
          reader.readAsDataURL(file);
        }
        input.value = "";
      },

      updateVisuals: function (src, isVideo) {
        this.isVideoBg = isVideo;
        const bgImg = document.getElementById("vg-bg-img"), bgVid = document.getElementById("vg-bg-video");
        const cvImg = document.getElementById("vg-cover-img"), cvVid = document.getElementById("vg-cover-video");
        bgImg.style.display = "none"; bgVid.style.display = "none"; cvImg.style.display = "none"; cvVid.style.display = "none";
        bgVid.pause(); cvVid.pause();
        if (isVideo) {
          bgVid.src = src; bgVid.style.display = "block"; bgVid.play().catch(() => {});
          cvVid.src = src; cvVid.style.display = "block"; cvVid.play().catch(() => {});
        } else {
          bgImg.src = src; bgImg.style.display = "block"; cvImg.src = src; cvImg.style.display = "block";
        }
      },

      open: async function (songData) {
        this.data = songData; this.customVisual = null;
        const modal = document.getElementById("vg-modal");
        this.updateVisuals(songData.cover || "https://via.placeholder.com/600", false);
        document.getElementById("vg-title").textContent = songData.name;
        document.getElementById("vg-artist").textContent = songData.artist;

        if (window.ap && window.ap.audio) {
            this.isPlaying = (window.currentPlayingId === songData.id) && !window.ap.audio.paused;
            this.updatePlayUI();
            
            if (this.apTimeHandler) window.ap.audio.removeEventListener('timeupdate', this.apTimeHandler);
            if (this.apPlayHandler) window.ap.audio.removeEventListener('play', this.apPlayHandler);
            if (this.apPauseHandler) window.ap.audio.removeEventListener('pause', this.apPauseHandler);
            if (this.apEndHandler) window.ap.audio.removeEventListener('ended', this.apEndHandler);

            this.apTimeHandler = () => this.syncLyrics();
            this.apPlayHandler = () => { 
                if (window.currentPlayingId === this.data.id) {
                    this.isPlaying = true; this.updatePlayUI(); this.initAudioContext(); this.startRealtimeVisualizer(); 
                    const b = document.getElementById("vg-bg-video"), c = document.getElementById("vg-cover-video");
                    if(b?.style.display !== 'none') b.play().catch(()=>{}); if(c?.style.display !== 'none') c.play().catch(()=>{});
                    document.getElementById("vg-bg-img")?.classList.add("playing");
                }
            };
            this.apPauseHandler = () => { 
                this.isPlaying = false; this.updatePlayUI(); this.stopRealtimeVisualizer(); 
                const b = document.getElementById("vg-bg-video"), c = document.getElementById("vg-cover-video");
                if(b?.style.display !== 'none') b.pause(); if(c?.style.display !== 'none') c.pause();
                document.getElementById("vg-bg-img")?.classList.remove("playing");
            };
            this.apEndHandler = () => { this.isPlaying = false; this.updatePlayUI(); this.stopRealtimeVisualizer(); };

            window.ap.audio.addEventListener('timeupdate', this.apTimeHandler);
            window.ap.audio.addEventListener('play', this.apPlayHandler);
            window.ap.audio.addEventListener('pause', this.apPauseHandler);
            window.ap.audio.addEventListener('ended', this.apEndHandler);
        }
        
        const sb = document.getElementById('vg-seek-bar');
        sb.oninput = (e) => {
            this.isDraggingProgress = true;
            let dur = (window.currentPlayingId === this.data.id && window.ap && window.ap.audio) 
                      ? window.ap.audio.duration 
                      : (this.data.duration || 0);
            if (dur > 0) {
                const targetTime = (e.target.value / 100) * dur;
                document.getElementById('vg-time-current').textContent = this.formatTime(targetTime);
            }
        };
        sb.onchange = (e) => {
            this.isDraggingProgress = false;
            if (window.currentPlayingId === this.data.id) {
                if (window.ap && window.ap.audio && window.ap.audio.duration) {
                    window.ap.seek((e.target.value / 100) * window.ap.audio.duration);
                    this.syncLyrics();
                    if (window.ap.audio.paused) window.ap.play();
                }
            } else {
                if (typeof window.playAllAndJumpToId === 'function') {
                    window.playAllAndJumpToId(this.data.id);
                    const onCanPlay = () => {
                        if (window.ap && window.ap.audio && window.ap.audio.duration) {
                            window.ap.seek((e.target.value / 100) * window.ap.audio.duration);
                        }
                        window.ap.audio.removeEventListener('canplay', onCanPlay);
                    };
                    window.ap.audio.addEventListener('canplay', onCanPlay);
                }
            }
        };

        this.reset(); this.loadLyrics(songData); modal.style.display = "flex";
        requestAnimationFrame(() => { modal.classList.add("active"); this.initRealtimeCanvas(); if (this.isPlaying) this.apPlayHandler(); });
      },

      close: function () {
        this.stopRealtimeVisualizer();
        if (window.ap && window.ap.audio) {
            window.ap.audio.removeEventListener('timeupdate', this.apTimeHandler);
            window.ap.audio.removeEventListener('play', this.apPlayHandler);
            window.ap.audio.removeEventListener('pause', this.apPauseHandler);
            window.ap.audio.removeEventListener('ended', this.apEndHandler);
        }
        if (this.resizeObserver) {
            this.resizeObserver.disconnect();
            this.resizeObserver = null;
        }
        const m = document.getElementById("vg-modal"); m.classList.remove("active");
        setTimeout(() => { m.style.display = "none"; this.reset(); }, 500);
      },

      reset: function () {
        document.getElementById("vg-status-loading").style.display = "none";
        document.getElementById("vg-status-success").style.display = "none";
        document.getElementById("vg-ui-container").classList.remove("vg-rendering-hide");
      },

      loadLyrics: function (song) {
        const box = document.getElementById("vg-lyrics"); box.innerHTML = '<p style="padding-top:100px; color:rgba(255,255,255,0.8);">Loading...</p>';
        this.lyricTimes = []; this.lyricRaw = [];
        fetch(`${window.API_ROOT}/lyric?id=${encodeURIComponent(song.id)}&source=${song.source}`).then(r => r.text()).then(text => {
          box.innerHTML = ""; const lines = text.split("\n"), re = /\[(\d{2}):(\d{2})\.(\d{2,3})\]/;
          lines.forEach(line => {
            const m = line.match(re), c = line.replace(/\[.*?\]/g, "").trim();
            if (m && c) {
              const t = parseInt(m[1]) * 60 + parseInt(m[2]) + parseInt(m[3]) / (m[3].length === 2 ? 100 : 1000);
              this.lyricTimes.push({ time: t, content: c }); this.lyricRaw.push({ time: t, text: c });
              const d = document.createElement("div"); d.className = "vg-line"; d.textContent = c;
              d.onclick = () => { 
                  if (window.currentPlayingId === this.data.id) {
                      if (window.ap && window.ap.audio) {
                          window.ap.seek(t);
                          if (window.ap.audio.paused) window.ap.play();
                      }
                  } else {
                      if (typeof window.playAllAndJumpToId === 'function') {
                          window.playAllAndJumpToId(this.data.id);
                          const onCanPlay = () => {
                              if (window.ap && window.ap.audio) window.ap.seek(t);
                              window.ap.audio.removeEventListener('canplay', onCanPlay);
                          };
                          window.ap.audio.addEventListener('canplay', onCanPlay);
                      }
                  }
              };
              box.appendChild(d);
            }
          });
          if (this.lyricTimes.length === 0) box.innerHTML = '<p style="padding-top:100px; color:rgba(255,255,255,0.5);">纯音乐 / 无歌词</p>';
        });
      },
  
      initRealtimeCanvas: function() {
          const c = document.getElementById('vg-visualizer'); c.innerHTML = '<canvas id="vg-rt-canvas"></canvas>';
          this.rtCanvas = document.getElementById('vg-rt-canvas'); this.rtCtx = this.rtCanvas.getContext('2d');
          const rz = () => { this.rtCanvas.width = c.offsetWidth; this.rtCanvas.height = c.offsetHeight; }; rz();
          if(!this.resizeObserver) { this.resizeObserver = new ResizeObserver(rz); this.resizeObserver.observe(c); }
      },

      initAudioContext: function() {
          if (!this.audioCtx) this.audioCtx = new (window.AudioContext || window.webkitAudioContext)();
          if (this.audioCtx.state === 'suspended') this.audioCtx.resume();
          if (!this.analyser) { this.analyser = this.audioCtx.createAnalyser(); this.analyser.fftSize = 256; this.analyser.smoothingTimeConstant = 0.65; }
      },
      connectAudioSource: function() {
          if (!window.ap || !window.ap.audio || !this.analyser) return; 
          if (this.sourceNode) return; 
          try { this.sourceNode = this.audioCtx.createMediaElementSource(window.ap.audio); this.sourceNode.connect(this.analyser); this.analyser.connect(this.audioCtx.destination); } catch(e) {}
      },
      startRealtimeVisualizer: function() {
          this.initAudioContext(); this.connectAudioSource();
          const dataArray = new Uint8Array(this.analyser.frequencyBinCount);
          
          const animate = () => {
              if (!this.isPlaying) return; this.animationId = requestAnimationFrame(animate);
              this.analyser.getByteFrequencyData(dataArray);
              const visResult = processVisualizerBars(dataArray);
              this.rtCtx.clearRect(0, 0, this.rtCanvas.width, this.rtCanvas.height);
              const dw = document.querySelector('.vg-disc-wrap');
              const radius = dw ? (dw.offsetWidth / 2 + 2) : (this.rtCanvas.width / 2 * 0.25);
              drawVisualizerRings(this.rtCtx, this.rtCanvas.width/2, this.rtCanvas.height/2, radius, visResult.heights);
          };
          animate();
      },
      stopRealtimeVisualizer: function() { if (this.animationId) cancelAnimationFrame(this.animationId); },
  
      // 【核心功能】：点击生成视频，打开新窗口进行无感渲染
      startRendering: function () {
        if (!this.data) return;
        
        window.__vgRenderData = {
            id: this.data.id,
            source: this.data.source,
            name: this.data.name,
            artist: this.data.artist,
            rawCover: this.customVisual || this.data.cover || "https://via.placeholder.com/600",
            isVideoBg: this.isVideoBg,
            lyricRaw: this.lyricRaw,
            apiRoot: window.API_ROOT
        };

        const renderWin = window.open(window.API_ROOT + '/render', '_blank');
        if (!renderWin) {
            alert("渲染页面被浏览器拦截，请允许弹出窗口！");
            return;
        }
      },
  
      togglePlay: function () {
        if (!this.data || !window.ap) return;
        if (window.currentPlayingId === this.data.id) window.ap.toggle();
        else if (typeof window.playAllAndJumpToId === 'function') window.playAllAndJumpToId(this.data.id);
      },
      updatePlayUI: function () {
        const i = document.querySelector("#vg-play-toggle i"), m = document.getElementById("vg-modal");
        if (i) i.className = this.isPlaying ? "fa-solid fa-pause" : "fa-solid fa-play";
        if (m) this.isPlaying ? m.classList.add("playing") : m.classList.remove("playing");
      },
      syncLyrics: function () {
        if (!window.ap || !window.ap.audio || window.currentPlayingId !== this.data.id) return;
        const ct = window.ap.audio.currentTime, dur = window.ap.audio.duration || 0;
        
        if (!this.isDraggingProgress) { 
            const sb = document.getElementById('vg-seek-bar'); 
            if(dur > 0){ 
                sb.value = (ct/dur)*100; 
                document.getElementById('vg-time-current').textContent = this.formatTime(ct); 
                document.getElementById('vg-time-total').textContent = this.formatTime(dur); 
            } 
        }
        
        if (this.lyricTimes.length === 0 || this.isUserScrolling) return;
        let active = -1; for (let i = 0; i < this.lyricTimes.length; i++) { if (ct >= this.lyricTimes[i].time) active = i; else break; }
        if (active !== -1 && active !== this.lastActiveIndex) {
          const ls = document.querySelectorAll(".vg-line"); ls.forEach(l => l.classList.remove("active"));
          const al = ls[active]; if (al) { al.classList.add("active"); if (!this.isUserScrolling) al.scrollIntoView({ behavior: "smooth", block: "center" }); }
          this.lastActiveIndex = active;
        }
      },
    };
})();