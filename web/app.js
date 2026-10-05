// FarsiForge Web UI — Frontend logic
// Communicates with the Go backend via REST API

const API = '';
let currentProject = null;
let gameInfo = null;
let allEntries = [];
let filteredEntries = [];

// ─── Navigation ────────────────────────────────────────────────────

document.querySelectorAll('.step').forEach(btn => {
    btn.addEventListener('click', () => {
        const step = btn.dataset.step;
        if (canNavigateTo(step)) {
            goToStep(step);
        }
    });
});

function canNavigateTo(step) {
    // Basic navigation guard
    if (step === 'detect') return true;
    if (step === 'extract') return gameInfo != null;
    if (step === 'translate') return currentProject != null;
    if (step === 'inject') return currentProject != null;
    if (step === 'installer') return currentProject != null;
    return true;
}

function goToStep(step) {
    document.querySelectorAll('.step').forEach(s => s.classList.remove('active'));
    document.querySelector(`.step[data-step="${step}"]`).classList.add('active');

    document.querySelectorAll('.view').forEach(v => v.classList.remove('active'));
    document.getElementById(`view-${step}`).classList.add('active');

    if (step === 'translate' && currentProject) {
        renderTranslationTable();
    }
}

// ─── Step 1: Detect ────────────────────────────────────────────────

async function browsePath() {
    // Can't access filesystem directly from browser — prompt user
    const input = document.getElementById('game-path');
    if (!input.value) {
        input.value = prompt('مسیر پوشه بازی را وارد کنید:', 'D:\\SteamLibrary\\steamapps\\common\\');
    }
}

async function detectEngine() {
    const path = document.getElementById('game-path').value.trim();
    if (!path) {
        alert('لطفاً مسیر پوشه بازی را وارد کنید');
        return;
    }

    showLoading('detect-result', true);
    try {
        const res = await fetch('/api/detect', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({ path })
        });
        const data = await res.json();
        gameInfo = data;
        showDetectResult(data);
    } catch (err) {
        alert('خطا در تشخیص: ' + err.message);
    }
}

function showDetectResult(data) {
    const badge = engineBadge(data.engine);
    const html = `
        <div class="detect-grid">
            <div><strong>موتور:</strong> ${badge}</div>
            <div><strong>نسخه:</strong> ${data.version || 'نامشخص'}</div>
            <div><strong>backend:</strong> ${data.backend || '-'}</div>
            <div><strong>بازی:</strong> ${data.game_name || '-'}</div>
            <div><strong>فایل اجرایی:</strong> ${data.game_exe || '-'}</div>
            <div><strong>اطمینان:</strong> ${(data.confidence * 100).toFixed(0)}%</div>
        </div>
        ${data.notes ? '<div class="notes"><strong>یادداشت‌ها:</strong><ul>' + 
            data.notes.map(n => `<li>${n}</li>`).join('') + '</ul></div>' : ''}
    `;
    document.getElementById('detect-info').innerHTML = html;
    document.getElementById('detect-result').classList.remove('hidden');
    
    // Also populate extract info
    document.getElementById('extract-info').innerHTML = 
        `<p>بازی: <strong>${data.game_name}</strong> | موتور: ${badge}</p>`;
}

function engineBadge(engine) {
    const classes = {
        'unity': 'engine-unity', 'unreal': 'engine-unreal',
        'godot': 'engine-godot', 'rpgmaker': 'engine-rpgmaker',
        'source': 'engine-source', 'goldsrc': 'engine-source',
        'gamemaker': 'engine-other', 'renpy': 'engine-other',
        'adobe_air': 'engine-other', 'custom': 'engine-other'
    };
    const cls = classes[engine] || 'engine-other';
    const names = {
        'unity': 'Unity', 'unreal': 'Unreal Engine', 'godot': 'Godot',
        'rpgmaker': 'RPG Maker', 'source': 'Source', 'goldsrc': 'GoldSrc',
        'gamemaker': 'GameMaker', 'renpy': "Ren'Py", 'adobe_air': 'Adobe AIR',
        'custom': 'Custom/Generic'
    };
    return `<span class="engine-badge ${cls}">${names[engine] || engine}</span>`;
}

// ─── Step 2: Extract ───────────────────────────────────────────────

async function extractStrings() {
    if (!gameInfo) { alert('ابتدا بازی را تشخیص دهید'); return; }

    document.getElementById('extract-progress').classList.remove('hidden');
    document.getElementById('extract-result').classList.add('hidden');

    try {
        const res = await fetch('/api/extract', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(gameInfo)
        });
        const data = await res.json();

        if (data.error) {
            alert('خطا در استخراج: ' + data.error);
        } else {
            currentProject = data.project;
            allEntries = data.entries || [];
            showExtractResult(data);
            updateStats();
        }
    } catch (err) {
        alert('خطا: ' + err.message);
    }

    document.getElementById('extract-progress').classList.add('hidden');
}

function showExtractResult(data) {
    const html = `
        <div class="stats-grid">
            <div class="stat-card"><span class="stat-num">${data.entry_count}</span><span>متن استخراج شده</span></div>
            <div class="stat-card"><span class="stat-num">${data.file_count}</span><span>فایل پردازش شده</span></div>
        </div>
    `;
    document.getElementById('extract-stats').innerHTML = html;
    document.getElementById('extract-result').classList.remove('hidden');
}

// ─── Step 3: Translate ─────────────────────────────────────────────

function renderTranslationTable() {
    const tbody = document.getElementById('translation-body');
    filteredEntries = allEntries.filter(e => matchesFilter(e));
    
    tbody.innerHTML = filteredEntries.slice(0, 500).map((e, i) => `
        <tr data-id="${e.ID}">
            <td>${i + 1}</td>
            <td class="source-cell" title="${escHtml(e.Source)}">${escHtml(truncate(e.Source, 50))}</td>
            <td class="translation-cell">
                <input type="text" class="translation-input" value="${escAttr(e.Translation || '')}" 
                       oninput="updateTranslation('${e.ID}', this.value)"
                       placeholder="ترجمه را وارد کنید...">
            </td>
            <td class="preview-cell">${escHtml(previewPersian(e.Translation))}</td>
            <td><span class="status-badge status-${e.Status}">${statusLabel(e.Status)}</span></td>
            <td>${e.Context || '-'}</td>
        </tr>
    `).join('');

    if (filteredEntries.length > 500) {
        tbody.innerHTML += `<tr><td colspan="6" style="text-align:center;color:var(--text-dim)">... ${filteredEntries.length - 500} مورد دیگر (برای دیدن همه، جستجو کنید)</td></tr>`;
    }
}

function matchesFilter(e) {
    const search = document.getElementById('search-box').value.toLowerCase();
    const status = document.getElementById('filter-status').value;
    
    if (status && e.Status !== status) return false;
    if (search) {
        return e.Source.toLowerCase().includes(search) ||
               (e.Translation || '').toLowerCase().includes(search);
    }
    return true;
}

function filterEntries() {
    renderTranslationTable();
}

async function updateTranslation(id, value) {
    const entry = allEntries.find(e => e.ID === id);
    if (!entry) return;
    
    entry.Translation = value;
    entry.Status = value ? 'translated' : 'untranslated';
    
    // Update preview
    const row = document.querySelector(`tr[data-id="${id}"]`);
    if (row) {
        const previewCell = row.querySelector('.preview-cell');
        previewCell.textContent = previewPersian(value);
        const statusCell = row.querySelector('.status-badge');
        statusCell.className = `status-badge status-${entry.Status}`;
        statusCell.textContent = statusLabel(entry.Status);
    }
    
    updateStats();
    
    // Debounced save to backend
    clearTimeout(window._saveTimer);
    window._saveTimer = setTimeout(() => saveTranslation(id, value), 500);
}

async function saveTranslation(id, value) {
    try {
        await fetch('/api/translate', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({ id, translation: value })
        });
    } catch (err) {
        console.error('Save failed:', err);
    }
}

function previewPersian(text) {
    if (!text) return '';
    // Simple preview: just show the text (the actual shaping happens during injection)
    return truncate(text, 40);
}

// ─── Step 4: Inject ────────────────────────────────────────────────

async function injectTranslations() {
    if (!currentProject || !gameInfo) { alert('ابتدا استخراج و ترجمه انجام دهید'); return; }

    document.getElementById('inject-progress').classList.remove('hidden');
    document.getElementById('inject-result').classList.add('hidden');

    const opts = {
        reshape: document.getElementById('opt-reshape').checked,
        bidi_reorder: document.getElementById('opt-bidi').checked,
        fix_yeh: document.getElementById('opt-yeh').checked,
        persian_digits: document.getElementById('opt-digits').checked,
    };

    try {
        const res = await fetch('/api/inject', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({ game_info: gameInfo, options: opts })
        });
        const data = await res.json();

        if (data.error) {
            alert('خطا در تزریق: ' + data.error);
        } else {
            showInjectResult(data);
        }
    } catch (err) {
        alert('خطا: ' + err.message);
    }

    document.getElementById('inject-progress').classList.add('hidden');
}

function showInjectResult(data) {
    const html = `
        <p>✅ تزریق با موفقیت انجام شد!</p>
        <p>تعداد فایل‌های تغییر یافته: <strong>${data.modified_count}</strong></p>
        ${data.modified_files ? '<details><summary>فایل‌ها</summary><ul>' + 
            data.modified_files.map(f => `<li>${f}</li>`).join('') + '</ul></details>' : ''}
    `;
    document.getElementById('inject-info').innerHTML = html;
    document.getElementById('inject-result').classList.remove('hidden');
}

// ─── Step 5: Installer ─────────────────────────────────────────────

async function buildInstaller() {
    if (!currentProject || !gameInfo) { alert('ابتدا مراحل قبل را کامل کنید'); return; }

    document.getElementById('installer-progress').classList.remove('hidden');
    document.getElementById('installer-result').classList.add('hidden');

    const cfg = {
        patch_name: document.getElementById('patch-name').value || 'فارسی‌ساز',
        author: document.getElementById('patch-author').value || 'FarsiForge',
        description: document.getElementById('patch-desc').value || '',
        output_dir: document.getElementById('output-dir').value || '',
    };

    try {
        const res = await fetch('/api/build-installer', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(cfg)
        });
        const data = await res.json();

        if (data.error) {
            alert('خطا: ' + data.error);
        } else {
            document.getElementById('installer-info').innerHTML = `
                <p>✅ نصاب با موفقیت ساخته شد!</p>
                <p>مسیر خروجی: <strong>${data.output_dir}</strong></p>
                <p>فایل نصاب: <strong>${data.installer_path}</strong></p>
                <p>تعداد فایل‌های پچ: <strong>${data.file_count}</strong></p>
            `;
            document.getElementById('installer-result').classList.remove('hidden');
        }
    } catch (err) {
        alert('خطا: ' + err.message);
    }

    document.getElementById('installer-progress').classList.add('hidden');
}

// ─── Export/Import ─────────────────────────────────────────────────

async function exportXLSX() {
    window.location.href = '/api/export?format=xlsx';
}

async function exportCSV() {
    window.location.href = '/api/export?format=csv';
}

function importFile() {
    document.getElementById('import-file').click();
}

async function handleImport(event) {
    const file = event.target.files[0];
    if (!file) return;

    const formData = new FormData();
    formData.append('file', file);

    try {
        const res = await fetch('/api/import', { method: 'POST', body: formData });
        const data = await res.json();
        if (data.error) {
            alert('خطا: ' + data.error);
        } else {
            alert(`${data.imported} ترجمه ایمپورت شد`);
            // Reload entries
            const projRes = await fetch('/api/project');
            const projData = await projRes.json();
            currentProject = projData;
            allEntries = projData.entries || [];
            renderTranslationTable();
            updateStats();
        }
    } catch (err) {
        alert('خطا: ' + err.message);
    }
}

// ─── Utilities ─────────────────────────────────────────────────────

function updateStats() {
    const total = allEntries.length;
    const translated = allEntries.filter(e => e.Status === 'translated' || e.Status === 'approved').length;
    const remaining = total - translated;

    document.getElementById('stat-total').textContent = total;
    document.getElementById('stat-translated').textContent = translated;
    document.getElementById('stat-remaining').textContent = remaining;
    document.getElementById('stats-mini').classList.remove('hidden');
}

function showLoading(id, show) {
    const el = document.getElementById(id);
    if (show) el.classList.add('hidden');
    else el.classList.remove('hidden');
}

function statusLabel(s) {
    const labels = {
        'untranslated': 'ترجمه نشده',
        'translated': 'ترجمه شده',
        'approved': 'تایید شده',
        'skipped': 'رد شده'
    };
    return labels[s] || s;
}

function truncate(s, n) {
    if (!s) return '';
    return s.length > n ? s.substring(0, n) + '...' : s;
}

function escHtml(s) {
    if (!s) return '';
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function escAttr(s) {
    if (!s) return '';
    return s.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

// ─── Init ──────────────────────────────────────────────────────────

// Load tools status on startup
fetch('/api/tools').then(r => r.json()).then(data => {
    console.log('Tools available:', data);
}).catch(() => {});

// Set default output dir
document.getElementById('output-dir').value = 'D:\\FarsiForge\\output';
