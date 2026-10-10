// CODETEST - Frontend Engine
(function() {
    'use strict';

    // DOM Elements
    const codeEditor = document.getElementById('codeEditor');
    const lineGutter = document.getElementById('lineGutter');
    const statLines = document.getElementById('statLines');
    const statChars = document.getElementById('statChars');
    const statLangBadge = document.getElementById('statLangBadge');
    const languageSelect = document.getElementById('languageSelect');
    const presetSelect = document.getElementById('presetSelect');
    const analyzeBtn = document.getElementById('analyzeBtn');
    const clearBtn = document.getElementById('clearBtn');
    const uploadBtn = document.getElementById('uploadBtn');
    const fileInput = document.getElementById('fileInput');
    const resultsPanel = document.getElementById('resultsPanel');
    const issuesContainer = document.getElementById('issuesContainer');
    const summaryPills = document.getElementById('summaryPills');
    const togglePanelBtn = document.getElementById('togglePanelBtn');
    const countAll = document.getElementById('countAll');
    const countErrors = document.getElementById('countErrors');
    const countWarnings = document.getElementById('countWarnings');
    const countInfos = document.getElementById('countInfos');
    const filterBtns = document.querySelectorAll('.filter-btn');
    const toast = document.getElementById('toast');
    const pixelCanvas = document.getElementById('pixelCanvas');

    let currentIssues = [];
    let currentFilter = 'all';
    let presetsCache = {};

    // Initialize
    function init() {
        const requiredElements = [
            ['codeEditor', codeEditor],
            ['lineGutter', lineGutter],
            ['statLines', statLines],
            ['statChars', statChars],
            ['statLangBadge', statLangBadge],
            ['languageSelect', languageSelect],
            ['presetSelect', presetSelect],
            ['analyzeBtn', analyzeBtn],
            ['clearBtn', clearBtn],
            ['uploadBtn', uploadBtn],
            ['fileInput', fileInput],
            ['resultsPanel', resultsPanel],
            ['issuesContainer', issuesContainer],
            ['summaryPills', summaryPills],
            ['togglePanelBtn', togglePanelBtn],
            ['countAll', countAll],
            ['countErrors', countErrors],
            ['countWarnings', countWarnings],
            ['countInfos', countInfos],
            ['toast', toast]
        ];
        const missingElements = requiredElements.filter(([, element]) => !element).map(([id]) => id);
        if (missingElements.length > 0 || filterBtns.length === 0) {
            console.error('CODETEST initialization failed: missing required UI elements.', {
                elementIds: missingElements,
                filterButtons: filterBtns.length
            });
            return;
        }

        drawPixelAvatar();
        updateLineNumbers();
        bindEvents();
        loadPresets();
    }

    // Draw 8-Bit Pixel Sprite on Canvas
    function drawPixelAvatar() {
        if (!pixelCanvas) return;
        const ctx = pixelCanvas.getContext('2d');
        if (!ctx) return;
        const s = 4; // pixel block size (12x12 grid = 48x48)
        
        // 8-bit Bug Hunter Sprite
        const sprite = [
            "  GGGGGGGG  ",
            " GGGGGGGGGG ",
            "GGGWWGGWWGGG",
            "GGGBBGGGBBGG",
            "GGGGGGGGGGGG",
            " GGGGRRGGGG ",
            "  GGGRRGGG  ",
            " G GGGGGG G ",
            "G  GGGGGG  G",
            "G  G GG G  G",
            "   G    G   ",
            "   GG  GG   "
        ];

        const colorMap = {
            'G': '#00ff66',
            'W': '#ffffff',
            'B': '#0b0f19',
            'R': '#ff3355',
            ' ': 'transparent'
        };

        ctx.clearRect(0, 0, 48, 48);
        for (let r = 0; r < sprite.length; r++) {
            const row = sprite[r];
            for (let c = 0; c < row.length; c++) {
                const char = row[c];
                if (char !== ' ') {
                    ctx.fillStyle = colorMap[char] || '#00ff66';
                    ctx.fillRect(c * s, r * s, s, s);
                }
            }
        }
    }

    // Update Line Numbers & Stats
    function updateLineNumbers() {
        const code = typeof codeEditor.value === 'string' ? codeEditor.value : '';
        const lines = code.split('\n');
        const count = lines.length;
        
        statLines.textContent = `Lines: ${count}`;
        statChars.textContent = `Chars: ${code.length}`;

        // Map errors to line numbers for gutter markers
        const errorLineSet = new Set();
        const warnLineSet = new Set();
        currentIssues.forEach(issue => {
            if (issue.severity === 'error') {
                errorLineSet.add(issue.line);
            } else if (issue.severity === 'warning') {
                warnLineSet.add(issue.line);
            }
        });

        let gutterHtml = '';
        for (let i = 1; i <= count; i++) {
            let cls = 'line-num';
            if (errorLineSet.has(i)) {
                cls += ' has-error';
            } else if (warnLineSet.has(i)) {
                cls += ' has-warning';
            }
            gutterHtml += `<div class="${cls}" data-line="${i}">${i}</div>`;
        }
        lineGutter.innerHTML = gutterHtml;
    }

    // Synchronize textarea and gutter scrolling
    function syncScroll() {
        lineGutter.scrollTop = codeEditor.scrollTop;
    }

    // Jump to Line in Editor
    function jumpToLine(targetLine) {
        const lines = codeEditor.value.split('\n');
        if (targetLine < 1 || targetLine > lines.length) return;

        let charPos = 0;
        for (let i = 0; i < targetLine - 1; i++) {
            charPos += lines[i].length + 1; // +1 for \n
        }

        codeEditor.focus();
        codeEditor.setSelectionRange(charPos, charPos + lines[targetLine - 1].length);

        // Approximate line height ~23.2px (1.45rem * 16px)
        const lineHeight = 23.2;
        const scrollTarget = (targetLine - 1) * lineHeight - (codeEditor.clientHeight / 3);
        codeEditor.scrollTop = Math.max(0, scrollTarget);
        syncScroll();

        // Highlight line in gutter
        const gutterNums = lineGutter.querySelectorAll('.line-num');
        gutterNums.forEach(el => {
            if (parseInt(el.getAttribute('data-line'), 10) === targetLine) {
                el.classList.add('line-flash');
                setTimeout(() => el.classList.remove('line-flash'), 1600);
            }
        });

        showToast(`JUMPED TO LINE ${targetLine}`);
    }

    // Show Notification Toast
    function showToast(msg) {
        if (!toast) {
            console.error('CODETEST notification:', msg);
            return;
        }
        toast.textContent = msg;
        toast.classList.add('show');
        setTimeout(() => toast.classList.remove('show'), 2000);
    }

    // Fetch Presets from Backend
    async function loadPresets() {
        try {
            const res = await fetch('/api/presets');
            if (res.ok) {
                const data = await res.json();
                if (!data || typeof data !== 'object' || Array.isArray(data)) {
                    throw new Error('Preset endpoint returned an invalid response.');
                }
                for (const [key, preset] of Object.entries(data)) {
                    if (!preset || typeof preset.code !== 'string' ||
                        typeof preset.language !== 'string' || typeof preset.title !== 'string') {
                        throw new Error(`Preset "${key}" is incomplete.`);
                    }
                }
                presetsCache = data;
            } else {
                throw new Error(`Preset endpoint returned HTTP ${res.status}.`);
            }
        } catch (error) {
            console.error('Could not load presets from backend.', error);
            showToast('PRESETS FAILED TO LOAD');
        }
    }

    // Apply Preset Code
    function applyPreset(key) {
        if (!key) return;
        const preset = presetsCache[key];
        if (preset) {
            codeEditor.value = preset.code;
            languageSelect.value = preset.language || 'auto';
            statLangBadge.textContent = (preset.language || 'AUTO').toUpperCase();
            updateLineNumbers();
            showToast(`LOADED ${preset.title.split(' ')[0]} PRESET`);
            runAnalysis();
        } else {
            showToast(`PRESET NOT AVAILABLE: ${key}`);
        }
    }

    function validateAnalysisResponse(data) {
        if (!data || typeof data !== 'object' || Array.isArray(data) || data.success !== true) {
            throw new Error('Analysis endpoint returned an invalid response.');
        }
        if (typeof data.language !== 'string' || !Number.isInteger(data.total_lines) || data.total_lines < 0) {
            throw new Error('Analysis response contains invalid language or line-count information.');
        }
        if (!data.summary || typeof data.summary !== 'object' || Array.isArray(data.summary)) {
            throw new Error('Analysis response is missing its summary.');
        }

        const summary = data.summary;
        for (const key of ['total_errors', 'total_warnings', 'total_infos']) {
            if (!Number.isInteger(summary[key]) || summary[key] < 0) {
                throw new Error(`Analysis response contains an invalid "${key}" count.`);
            }
        }
        if (typeof summary.status !== 'string') {
            throw new Error('Analysis response contains an invalid summary status.');
        }
        if (typeof summary.execution_time_ms !== 'number' || !Number.isFinite(summary.execution_time_ms)) {
            throw new Error('Analysis response contains an invalid execution time.');
        }

        const issuesWereNull = data.issues === null;
        const issues = issuesWereNull ? [] : data.issues;
        if (!Array.isArray(issues)) {
            throw new Error('Analysis response "issues" must be an array.');
        }
        if (issuesWereNull && summary.total_errors + summary.total_warnings + summary.total_infos > 0) {
            throw new Error('Analysis response has null issues but reports nonzero issue counts.');
        }

        for (const [index, issue] of issues.entries()) {
            if (!issue || typeof issue !== 'object' || Array.isArray(issue) ||
                !Number.isInteger(issue.line) || issue.line < 1 ||
                !['error', 'warning', 'info'].includes(issue.severity) ||
                typeof issue.category !== 'string' ||
                typeof issue.rule !== 'string' ||
                typeof issue.message !== 'string') {
                throw new Error(`Analysis response contains an invalid issue at index ${index}.`);
            }
            issue.snippet = typeof issue.snippet === 'string' ? issue.snippet : '';
            issue.suggestion = typeof issue.suggestion === 'string' ? issue.suggestion : '';
        }
        const issueCounts = issues.reduce((counts, issue) => {
            counts[issue.severity]++;
            return counts;
        }, { error: 0, warning: 0, info: 0 });
        if (issueCounts.error !== summary.total_errors ||
            issueCounts.warning !== summary.total_warnings ||
            issueCounts.info !== summary.total_infos) {
            throw new Error('Analysis response issue list does not match its summary counts.');
        }

        return { ...data, issues };
    }

    // Run Code Analysis Request
    async function runAnalysis() {
        const code = typeof codeEditor.value === 'string' ? codeEditor.value : '';
        const isNullApp = !code.trim();
        if (isNullApp) {
            showToast('TESTING NULL / EMPTY APPLICATION...');
        }

        analyzeBtn.disabled = true;
        analyzeBtn.innerHTML = '<span class="btn-icon">⌛</span> TESTING...';

        try {
            const res = await fetch('/api/analyze', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    code: code,
                    language: languageSelect.value
                })
            });

            if (!res.ok) {
                throw new Error(`Server returned HTTP ${res.status}`);
            }

            const data = validateAnalysisResponse(await res.json());
            currentIssues = data.issues;

            // Update stats badge
            statLangBadge.textContent = (data.language || 'AUTO').toUpperCase();

            // Render Results
            renderResults(data);
            updateLineNumbers();

            // Expand panel if minimized
            resultsPanel.classList.remove('minimized');
            togglePanelBtn.textContent = '▼';

            if (data.summary.total_errors > 0) {
                showToast(`TEST COMPLETE: ${data.summary.total_errors} ERRORS`);
            } else if (data.summary.total_warnings > 0) {
                showToast(`TEST COMPLETE: ${data.summary.total_warnings} WARNINGS`);
            } else {
                showToast('DONE: NO ERRORS FOUND');
            }

        } catch (err) {
            console.error(err);
            showToast('ANALYSIS FAILED: ' + (err instanceof Error ? err.message : String(err)));
        } finally {
            analyzeBtn.disabled = false;
            analyzeBtn.innerHTML = '<span class="btn-icon">⚡</span> TEST CODE';
        }
    }

    // Render Results in Bottom Panel
    function renderResults(data) {
        const { issues, summary } = data;
        const total = issues.length;
        const errs = summary.total_errors || 0;
        const warns = summary.total_warnings || 0;
        const infos = summary.total_infos || 0;

        // Update counts
        countAll.textContent = total;
        countErrors.textContent = errs;
        countWarnings.textContent = warns;
        countInfos.textContent = infos;

        // Update Summary Pills
        let pillsHtml = '';
        if (errs > 0) {
            pillsHtml += `<span class="pill pill-errors">✖ ${errs} CRITICAL ERRORS</span>`;
        }
        if (warns > 0) {
            pillsHtml += `<span class="pill pill-warnings">▲ ${warns} WARNINGS</span>`;
        }
        if (errs === 0 && warns === 0) {
            pillsHtml += `<span class="pill pill-passed">✔ DONE: NO ERRORS FOUND</span>`;
        }
        pillsHtml += `<span class="pill pill-idle">⏱ ${summary.execution_time_ms.toFixed(2)} ms</span>`;
        summaryPills.innerHTML = pillsHtml;

        renderFilteredIssues();
    }

    // Filter and Render Issue Cards
    function renderFilteredIssues() {
        let filtered = currentIssues;
        if (currentFilter !== 'all') {
            filtered = currentIssues.filter(i => i.severity.toLowerCase() === currentFilter);
        }

        if (filtered.length === 0) {
            if (currentIssues.length === 0) {
                issuesContainer.innerHTML = `
                    <div class="empty-state">
                        <div class="pixel-radar"></div>
                        <p class="empty-title">DONE: NO ERRORS FOUND</p>
                        <p class="empty-desc">The selected language checks completed without finding errors or warnings.</p>
                    </div>`;
            } else {
                issuesContainer.innerHTML = `
                    <div class="empty-state">
                        <p class="empty-title">NO ${currentFilter.toUpperCase()}S DETECTED</p>
                        <p class="empty-desc">Switch filter tab to see other issues.</p>
                    </div>`;
            }
            return;
        }

        let html = '';
        filtered.forEach((issue) => {
            const sevClass = `severity-${issue.severity.toLowerCase()}`;
            const lineNum = issue.line;
            const snippet = escapeHtml(issue.snippet || '');

            html += `
                <div class="issue-card ${sevClass}">
                    <div class="issue-header">
                        <div class="issue-badges">
                            <!-- Prominent Line Number Badge -->
                            <span class="line-badge" onclick="window.codetestJump(${lineNum})">
                                <span>[ LINE</span> <strong>${lineNum}</strong> <span>]</span>
                            </span>
                            <span class="category-tag">${escapeHtml(issue.category)}</span>
                            <span class="rule-code">${escapeHtml(issue.rule)}</span>
                        </div>
                        <button class="jump-btn" onclick="window.codetestJump(${lineNum})">GO TO LINE ${lineNum} ➔</button>
                    </div>
                    <div class="issue-message">${escapeHtml(issue.message)}</div>
                    ${snippet ? `
                        <div class="issue-snippet-box">
                            <span class="snippet-num">L${lineNum}</span>
                            <code class="snippet-text">${snippet}</code>
                        </div>
                    ` : ''}
                    ${issue.suggestion ? `
                        <div class="issue-suggestion">
                            <span class="suggest-label">FIX:</span>
                            <span>${escapeHtml(issue.suggestion)}</span>
                        </div>
                    ` : ''}
                </div>
            `;
        });

        issuesContainer.innerHTML = html;
    }

    function escapeHtml(str) {
        if (!str) return '';
        return str
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#039;');
    }

    // Expose jump helper to global window for onclick handlers
    window.codetestJump = function(line) {
        jumpToLine(line);
    };

    // Bind Event Handlers
    function bindEvents() {
        // Textarea input
        codeEditor.addEventListener('input', () => {
            updateLineNumbers();
        });

        codeEditor.addEventListener('scroll', syncScroll);

        // Tab key support in textarea
        codeEditor.addEventListener('keydown', (e) => {
            if (e.key === 'Tab') {
                e.preventDefault();
                const start = codeEditor.selectionStart;
                const end = codeEditor.selectionEnd;
                codeEditor.value = codeEditor.value.substring(0, start) + '    ' + codeEditor.value.substring(end);
                codeEditor.selectionStart = codeEditor.selectionEnd = start + 4;
                updateLineNumbers();
            } else if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
                // Ctrl+Enter or Cmd+Enter shortcut
                e.preventDefault();
                runAnalysis();
            }
        });

        // Test Code Button
        analyzeBtn.addEventListener('click', runAnalysis);

        // Clear Button
        clearBtn.addEventListener('click', () => {
            codeEditor.value = '';
            currentIssues = [];
            updateLineNumbers();
            issuesContainer.innerHTML = `
                <div class="empty-state">
                    <div class="pixel-radar"></div>
                    <p class="empty-title">EDITOR CLEARED</p>
                    <p class="empty-desc">Paste code or select a preset to begin review.</p>
                </div>`;
            summaryPills.innerHTML = '<span class="pill pill-idle">RUN TEST TO DETECT ERRORS</span>';
            countAll.textContent = '0';
            countErrors.textContent = '0';
            countWarnings.textContent = '0';
            countInfos.textContent = '0';
            showToast('EDITOR CLEARED');
        });

        // Preset select
        presetSelect.addEventListener('change', (e) => {
            applyPreset(e.target.value);
        });

        // Filter buttons
        filterBtns.forEach(btn => {
            btn.addEventListener('click', () => {
                filterBtns.forEach(b => b.classList.remove('active'));
                btn.classList.add('active');
                currentFilter = btn.getAttribute('data-filter');
                renderFilteredIssues();
            });
        });

        // Toggle panel minimize
        togglePanelBtn.addEventListener('click', () => {
            resultsPanel.classList.toggle('minimized');
            togglePanelBtn.textContent = resultsPanel.classList.contains('minimized') ? '▲' : '▼';
        });

        // Upload Button
        uploadBtn.addEventListener('click', () => {
            fileInput.click();
        });

        fileInput.addEventListener('change', (e) => {
            const file = e.target && e.target.files && e.target.files[0];
            if (!file) return;

            const reader = new FileReader();
            reader.onload = (evt) => {
                if (!evt.target || typeof evt.target.result !== 'string') {
                    showToast('FILE READ FAILED: EXPECTED TEXT');
                    return;
                }
                codeEditor.value = evt.target.result;
                // Auto detect by extension
                const ext = file.name.split('.').pop().toLowerCase();
                const extMap = {
                    'py': 'python',
                    'js': 'javascript',
                    'ts': 'typescript',
                    'go': 'go',
                    'cpp': 'cpp',
                    'cc': 'cpp',
                    'cxx': 'cpp',
                    'c': 'c',
                    'java': 'java',
                    'cs': 'csharp',
                    'rs': 'rust',
                    'php': 'php',
                    'rb': 'ruby',
                    'kt': 'kotlin',
                    'swift': 'swift',
                    'sql': 'sql',
                    'html': 'html',
                    'htm': 'html',
                    'css': 'html'
                };
                if (extMap[ext]) {
                    languageSelect.value = extMap[ext];
                } else {
                    languageSelect.value = 'auto';
                }
                updateLineNumbers();
                showToast(`LOADED ${file.name}`);
                runAnalysis();
            };
            reader.onerror = () => showToast('FILE READ FAILED');
            reader.readAsText(file);
        });
    }

    // Run initialization on DOM load
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
