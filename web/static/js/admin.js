/**
 * SpeedTest Admin Console Controller
 */

let currentTestsPage = 1;
let currentBotsPage = 1;
let testsSearchTimer = null;
let botsSearchTimer = null;

document.addEventListener('DOMContentLoaded', () => {
    // Check if initial password change is mandatory
    const isDefault = document.body.getAttribute('data-is-default-password') === 'true';
    if (isDefault) {
        openChangePasswordModal();
    }

    loadStats();
    loadTests(1);
    loadBots(1);

    // Search input debounces
    const testsSearch = document.getElementById('tests-search');
    if (testsSearch) {
        testsSearch.addEventListener('input', () => {
            clearTimeout(testsSearchTimer);
            testsSearchTimer = setTimeout(() => { loadTests(1); }, 350);
        });
    }

    const botsSearch = document.getElementById('bots-search');
    if (botsSearch) {
        botsSearch.addEventListener('input', () => {
            clearTimeout(botsSearchTimer);
            botsSearchTimer = setTimeout(() => { loadBots(1); }, 350);
        });
    }
});

// Tab Switcher
function switchTab(tabId) {
    document.querySelectorAll('.tab-content').forEach(el => el.classList.add('hidden'));
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.classList.remove('bg-slate-800', 'text-cyan-400');
        btn.classList.add('text-slate-400');
    });

    const activeContent = document.getElementById('tab-' + tabId);
    const activeBtn = document.getElementById('tab-btn-' + tabId);

    if (activeContent) activeContent.classList.remove('hidden');
    if (activeBtn) {
        activeBtn.classList.remove('text-slate-400');
        activeBtn.classList.add('bg-slate-800', 'text-cyan-400');
    }
}

// ------------------------------------------------------------------
// 1. Dashboard Stats
// ------------------------------------------------------------------
async function loadStats() {
    try {
        const res = await fetch('/data/api/stats');
        if (!res.ok) return;
        const data = await res.json();

        document.getElementById('stat-total-tests').textContent = data.total_tests.toLocaleString();
        document.getElementById('stat-tests-24h').textContent = data.tests_last_24h.toLocaleString();
        document.getElementById('stat-avg-down').textContent = data.avg_download_mbps.toFixed(1) + ' Mbps';
        document.getElementById('stat-avg-up').textContent = data.avg_upload_mbps.toFixed(1) + ' Mbps';
        document.getElementById('stat-avg-ping').textContent = data.avg_ping_ms.toFixed(1) + ' ms';
        document.getElementById('stat-blocked-bots').textContent = data.blocked_bots.toLocaleString();

        const badge = document.getElementById('tab-bots-badge');
        if (badge) badge.textContent = data.blocked_bots.toLocaleString();

        if (data.fqdn) {
            const fqdnInput = document.getElementById('fqdn-input');
            if (fqdnInput && !fqdnInput.value) fqdnInput.value = data.fqdn;
            document.getElementById('overview-fqdn').textContent = data.fqdn;
        }

        const sslBadge = document.getElementById('overview-ssl-status');
        if (sslBadge) {
            if (data.ssl_status === 'active') {
                sslBadge.className = 'px-2 py-0.5 rounded text-[11px] bg-emerald-950 text-emerald-400 border border-emerald-800 font-semibold';
                sslBadge.textContent = 'Active (HTTPS)';
            } else if (data.ssl_status) {
                sslBadge.textContent = data.ssl_status;
            }
        }
    } catch (e) {
        console.error('Failed to load stats:', e);
    }
}

// ------------------------------------------------------------------
// 2. Speedtest Logs Table
// ------------------------------------------------------------------
async function loadTests(page = 1) {
    currentTestsPage = page;
    const searchVal = document.getElementById('tests-search') ? document.getElementById('tests-search').value : '';
    const tbody = document.getElementById('tests-table-body');

    try {
        const res = await fetch(`/data/api/tests?page=${page}&limit=25&search=${encodeURIComponent(searchVal)}`);
        if (!res.ok) throw new Error('Failed to load test logs');
        const data = await res.json();

        if (!data.results || data.results.length === 0) {
            tbody.innerHTML = `<tr><td colspan="9" class="py-8 text-center text-slate-500">No test records found.</td></tr>`;
            document.getElementById('tests-page-info').textContent = 'Showing 0 records';
            document.getElementById('tests-prev-btn').disabled = true;
            document.getElementById('tests-next-btn').disabled = true;
            return;
        }

        tbody.innerHTML = data.results.map(row => {
            const dateStr = new Date(row.timestamp).toLocaleString();
            const location = [row.city, row.country].filter(Boolean).join(', ') || 'Unknown';
            return `
                <tr class="hover:bg-slate-900/40 transition">
                    <td class="py-3 px-4 font-mono text-[11px] text-slate-400 whitespace-nowrap">${dateStr}</td>
                    <td class="py-3 px-4 font-mono font-semibold text-cyan-400 whitespace-nowrap">${escapeHTML(row.ip)}</td>
                    <td class="py-3 px-4 font-semibold text-emerald-400 whitespace-nowrap">${row.download_mbps.toFixed(2)} Mbps</td>
                    <td class="py-3 px-4 font-semibold text-purple-400 whitespace-nowrap">${row.upload_mbps.toFixed(2)} Mbps</td>
                    <td class="py-3 px-4 text-slate-300 whitespace-nowrap">${row.ping_ms.toFixed(1)} ms</td>
                    <td class="py-3 px-4 text-slate-300 whitespace-nowrap">${row.jitter_ms.toFixed(1)} ms</td>
                    <td class="py-3 px-4 text-slate-300 max-w-[150px] truncate" title="${escapeHTML(row.isp)}">${escapeHTML(row.isp || 'Unknown')}</td>
                    <td class="py-3 px-4 text-slate-400 whitespace-nowrap">${escapeHTML(location)}</td>
                    <td class="py-3 px-4 text-slate-500 max-w-[200px] truncate font-mono text-[10px]" title="${escapeHTML(row.user_agent)}">${escapeHTML(row.user_agent)}</td>
                </tr>
            `;
        }).join('');

        const totalPages = Math.ceil(data.total / 25) || 1;
        document.getElementById('tests-page-info').textContent = `Showing page ${data.page} of ${totalPages} (${data.total.toLocaleString()} total)`;
        document.getElementById('tests-prev-btn').disabled = (data.page <= 1);
        document.getElementById('tests-next-btn').disabled = (data.page >= totalPages);

    } catch (e) {
        tbody.innerHTML = `<tr><td colspan="9" class="py-8 text-center text-rose-400">Error loading records: ${e.message}</td></tr>`;
    }
}

function prevTestsPage() {
    if (currentTestsPage > 1) loadTests(currentTestsPage - 1);
}

function nextTestsPage() {
    loadTests(currentTestsPage + 1);
}

// ------------------------------------------------------------------
// 3. Blocked Bots Table (Up to 10,000 Rows)
// ------------------------------------------------------------------
async function loadBots(page = 1) {
    currentBotsPage = page;
    const searchVal = document.getElementById('bots-search') ? document.getElementById('bots-search').value : '';
    const tbody = document.getElementById('bots-table-body');

    try {
        const res = await fetch(`/data/api/bots?page=${page}&limit=25&search=${encodeURIComponent(searchVal)}`);
        if (!res.ok) throw new Error('Failed to load bot logs');
        const data = await res.json();

        if (!data.logs || data.logs.length === 0) {
            tbody.innerHTML = `<tr><td colspan="5" class="py-8 text-center text-slate-500">No blocked bot records found. Bot Guard is monitoring.</td></tr>`;
            document.getElementById('bots-page-info').textContent = 'Showing 0 records';
            document.getElementById('bots-prev-btn').disabled = true;
            document.getElementById('bots-next-btn').disabled = true;
            return;
        }

        tbody.innerHTML = data.logs.map(row => {
            const dateStr = new Date(row.timestamp).toLocaleString();
            return `
                <tr class="hover:bg-slate-900/40 transition">
                    <td class="py-3 px-4 font-mono text-[11px] text-slate-400 whitespace-nowrap">${dateStr}</td>
                    <td class="py-3 px-4 font-mono font-semibold text-rose-400 whitespace-nowrap">${escapeHTML(row.ip)}</td>
                    <td class="py-3 px-4 whitespace-nowrap">
                        <span class="px-2 py-0.5 rounded text-[11px] bg-rose-950 text-rose-300 border border-rose-800 font-medium">${escapeHTML(row.reason)}</span>
                    </td>
                    <td class="py-3 px-4 font-mono text-slate-400 whitespace-nowrap">${escapeHTML(row.path)}</td>
                    <td class="py-3 px-4 text-slate-500 max-w-[280px] truncate font-mono text-[10px]" title="${escapeHTML(row.user_agent)}">${escapeHTML(row.user_agent)}</td>
                </tr>
            `;
        }).join('');

        const totalPages = Math.ceil(data.total / 25) || 1;
        document.getElementById('bots-page-info').textContent = `Showing page ${data.page} of ${totalPages} (${data.total.toLocaleString()} total logs, capped at 10,000)`;
        document.getElementById('bots-prev-btn').disabled = (data.page <= 1);
        document.getElementById('bots-next-btn').disabled = (data.page >= totalPages);

    } catch (e) {
        tbody.innerHTML = `<tr><td colspan="5" class="py-8 text-center text-rose-400">Error loading bot records: ${e.message}</td></tr>`;
    }
}

function prevBotsPage() {
    if (currentBotsPage > 1) loadBots(currentBotsPage - 1);
}

function nextBotsPage() {
    loadBots(currentBotsPage + 1);
}

async function clearBots() {
    if (!confirm('Are you sure you want to clear all blocked bot logs?')) return;
    try {
        const res = await fetch('/data/api/clear-bots', { method: 'POST' });
        if (res.ok) {
            loadBots(1);
            loadStats();
        }
    } catch (e) {
        alert('Failed to clear bot logs.');
    }
}

// ------------------------------------------------------------------
// 4. Domain & Let's Encrypt SSL
// ------------------------------------------------------------------
async function saveDomainOnly() {
    const fqdn = document.getElementById('fqdn-input').value.trim();
    if (!fqdn) {
        alert('Please enter a valid domain name (FQDN).');
        return;
    }

    try {
        const res = await fetch('/data/api/settings/domain', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ fqdn })
        });
        const data = await res.json();
        if (res.ok) {
            alert(`Domain saved as "${data.fqdn}". You can now verify DNS and provision SSL.`);
            loadStats();
        } else {
            alert(data.error || 'Failed to save domain.');
        }
    } catch (e) {
        alert('Server communication error.');
    }
}

async function verifyDNSAndSSL() {
    const fqdn = document.getElementById('fqdn-input').value.trim();
    const btn = document.getElementById('verify-ssl-btn');
    const feedback = document.getElementById('ssl-feedback');

    if (!fqdn) {
        alert('Please specify the domain name (FQDN) first.');
        return;
    }

    btn.disabled = true;
    btn.innerHTML = `<svg class="animate-spin -ml-1 mr-2 h-4 w-4 text-white inline" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"></path></svg> Verifying DNS & Generating SSL...`;
    feedback.classList.add('hidden');

    try {
        const res = await fetch('/data/api/settings/verify-ssl', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ fqdn })
        });

        const data = await res.json();
        feedback.classList.remove('hidden');

        if (data.success) {
            feedback.className = 'mt-6 p-4 rounded-xl border bg-emerald-950/70 border-emerald-800 text-emerald-200 text-xs space-y-2';
            feedback.innerHTML = `
                <div class="font-bold flex items-center gap-1.5 text-emerald-300">
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path></svg>
                    <span>DNS Verified & Caddy SSL Provisioned</span>
                </div>
                <p>${escapeHTML(data.message)}</p>
                <div class="pt-1 text-[11px] text-emerald-400">
                    Target Domain: <strong>${escapeHTML(data.fqdn)}</strong> &bull; Resolved IPs: <strong>${(data.resolved_ips || []).join(', ')}</strong>
                </div>
                <div class="pt-2 text-slate-300">
                    You can now access your application via: <a href="https://${escapeHTML(data.fqdn)}" target="_blank" class="underline text-cyan-400 font-semibold">https://${escapeHTML(data.fqdn)}</a>
                </div>
            `;
            loadStats();
        } else {
            feedback.className = 'mt-6 p-4 rounded-xl border bg-rose-950/70 border-rose-800 text-rose-200 text-xs space-y-2';
            feedback.innerHTML = `
                <div class="font-bold flex items-center gap-1.5 text-rose-300">
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path></svg>
                    <span>DNS Verification Failed</span>
                </div>
                <p>${escapeHTML(data.message)}</p>
            `;
        }
    } catch (e) {
        feedback.className = 'mt-6 p-4 rounded-xl border bg-rose-950/70 border-rose-800 text-rose-200 text-xs';
        feedback.textContent = 'Failed to execute DNS check: ' + e.message;
        feedback.classList.remove('hidden');
    } finally {
        btn.disabled = false;
        btn.innerHTML = `<span>Verify DNS & Generate Let's Encrypt SSL</span>`;
    }
}

// ------------------------------------------------------------------
// 5. Password Management & Mandatory First Initialization Modal
// ------------------------------------------------------------------
function openChangePasswordModal() {
    const modal = document.getElementById('mandatory-pwd-modal');
    if (modal) modal.classList.remove('hidden');
}

async function handlePasswordChange(e) {
    e.preventDefault();
    const curr = document.getElementById('curr-pwd').value;
    const newPwd = document.getElementById('new-pwd').value;
    const confirmPwd = document.getElementById('confirm-pwd').value;
    const alertBox = document.getElementById('pwd-change-alert');

    if (newPwd !== confirmPwd) {
        alertBox.textContent = 'New passwords do not match.';
        alertBox.className = 'mb-4 p-3 rounded-xl border text-xs font-medium bg-rose-950 border-rose-800 text-rose-300';
        alertBox.classList.remove('hidden');
        return;
    }

    try {
        const res = await fetch('/data/api/change-password', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ current_password: curr, new_password: newPwd })
        });
        const data = await res.json();
        if (res.ok) {
            alertBox.textContent = 'Password changed successfully!';
            alertBox.className = 'mb-4 p-3 rounded-xl border text-xs font-medium bg-emerald-950 border-emerald-800 text-emerald-300';
            alertBox.classList.remove('hidden');
            document.getElementById('pwd-change-form').reset();
            const banner = document.getElementById('default-pass-banner');
            if (banner) banner.remove();
        } else {
            alertBox.textContent = data.error || 'Failed to update password.';
            alertBox.className = 'mb-4 p-3 rounded-xl border text-xs font-medium bg-rose-950 border-rose-800 text-rose-300';
            alertBox.classList.remove('hidden');
        }
    } catch (e) {
        alertBox.textContent = 'Server connection error.';
        alertBox.className = 'mb-4 p-3 rounded-xl border text-xs font-medium bg-rose-950 border-rose-800 text-rose-300';
        alertBox.classList.remove('hidden');
    }
}

async function handleModalPasswordChange(e) {
    e.preventDefault();
    const curr = document.getElementById('modal-curr-pwd').value;
    const newPwd = document.getElementById('modal-new-pwd').value;
    const confirmPwd = document.getElementById('modal-confirm-pwd').value;
    const alertBox = document.getElementById('modal-pwd-alert');

    if (newPwd !== confirmPwd) {
        alertBox.textContent = 'New passwords do not match.';
        alertBox.className = 'mb-4 p-3 rounded-xl border text-xs font-medium bg-rose-950 border-rose-800 text-rose-300';
        alertBox.classList.remove('hidden');
        return;
    }

    try {
        const res = await fetch('/data/api/change-password', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ current_password: curr, new_password: newPwd })
        });
        const data = await res.json();
        if (res.ok) {
            alert('Password changed successfully! You can now use the admin console safely.');
            document.getElementById('mandatory-pwd-modal').classList.add('hidden');
            const banner = document.getElementById('default-pass-banner');
            if (banner) banner.remove();
        } else {
            alertBox.textContent = data.error || 'Failed to update password.';
            alertBox.className = 'mb-4 p-3 rounded-xl border text-xs font-medium bg-rose-950 border-rose-800 text-rose-300';
            alertBox.classList.remove('hidden');
        }
    } catch (e) {
        alertBox.textContent = 'Server connection error.';
        alertBox.className = 'mb-4 p-3 rounded-xl border text-xs font-medium bg-rose-950 border-rose-800 text-rose-300';
        alertBox.classList.remove('hidden');
    }
}

function escapeHTML(str) {
    if (!str) return '';
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}
