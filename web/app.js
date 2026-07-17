(function () {
  'use strict';

  let currentPath = '';
  let currentPrefix = '';

  const fileList = document.getElementById('file-list');
  const breadcrumb = document.getElementById('breadcrumb');
  const prefixSearch = document.getElementById('prefix-search');
  const uploadStatus = document.getElementById('upload-status');
  const progressWrap = document.getElementById('upload-progress');
  const progressBar = document.getElementById('progress-bar');
  const progressText = document.getElementById('progress-text');

  function formatSize(bytes) {
    if (bytes === 0) return '—';
    const units = ['B', 'KB', 'MB', 'GB'];
    let i = 0;
    let n = bytes;
    while (n >= 1024 && i < units.length - 1) {
      n /= 1024;
      i++;
    }
    return n.toFixed(i > 0 ? 1 : 0) + ' ' + units[i];
  }

  function absoluteUrl(path) {
    return window.location.origin + path;
  }

  function setStatus(msg, type) {
    uploadStatus.textContent = msg;
    uploadStatus.className = 'status-msg' + (type ? ' ' + type : '');
  }

  function renderBreadcrumb() {
    const parts = currentPath ? currentPath.split('/') : [];
    let html = '<a data-path="">根目录</a>';
    let acc = '';
    parts.forEach(function (part) {
      acc = acc ? acc + '/' + part : part;
      const p = acc;
      html += '<span class="sep">/</span><a data-path="' + escapeAttr(p) + '">' + escapeHtml(part) + '</a>';
    });
    breadcrumb.innerHTML = html;
    breadcrumb.querySelectorAll('a').forEach(function (a) {
      a.addEventListener('click', function () {
        currentPath = a.getAttribute('data-path') || '';
        loadList();
      });
    });
  }

  function escapeHtml(s) {
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  function escapeAttr(s) {
    return escapeHtml(s);
  }

  async function loadList() {
    renderBreadcrumb();
    fileList.innerHTML = '<tr><td colspan="4" class="empty">加载中…</td></tr>';
    const params = new URLSearchParams();
    if (currentPath) params.set('path', currentPath);
    if (currentPrefix) params.set('prefix', currentPrefix);
    try {
      const res = await fetch('/api/files/list?' + params.toString());
      const data = await res.json();
      if (!data.success) {
        fileList.innerHTML = '<tr><td colspan="4" class="empty">' + escapeHtml(data.error || '加载失败') + '</td></tr>';
        return;
      }
      if (!data.entries || data.entries.length === 0) {
        fileList.innerHTML = '<tr><td colspan="4" class="empty">暂无文件</td></tr>';
        return;
      }
      fileList.innerHTML = data.entries.map(function (entry) {
        return renderRow(entry);
      }).join('');
      bindRowEvents();
    } catch {
      fileList.innerHTML = '<tr><td colspan="4" class="empty">网络错误</td></tr>';
    }
  }

  function renderRow(entry) {
    const isDir = entry.type === 'dir';
    const nameCell = isDir
      ? '<a class="dir-link" data-dir="' + escapeAttr(entry.name) + '">📁 ' + escapeHtml(entry.name) + '</a>'
      : escapeHtml(entry.name);
    const typeCell = isDir ? '目录' : '文件';
    const sizeCell = isDir ? '—' : formatSize(entry.size);
    let actions = '';
    if (!isDir && entry.url) {
      actions += '<a href="' + escapeAttr(entry.url) + '" target="_blank" rel="noopener">下载</a>';
      actions += '<button type="button" class="btn-copy" data-url="' + escapeAttr(entry.url) + '">复制 URL</button>';
    }
    if (!currentPath) {
      actions += '<button type="button" class="btn-danger" data-name="' + escapeAttr(entry.name) + '" data-type="' + entry.type + '">删除</button>';
    }
    return '<tr><td>' + nameCell + '</td><td>' + typeCell + '</td><td>' + sizeCell + '</td><td class="actions">' + actions + '</td></tr>';
  }

  function bindRowEvents() {
    fileList.querySelectorAll('.dir-link').forEach(function (el) {
      el.addEventListener('click', function () {
        const dir = el.getAttribute('data-dir');
        currentPath = currentPath ? currentPath + '/' + dir : dir;
        loadList();
      });
    });
    fileList.querySelectorAll('.btn-copy').forEach(function (btn) {
      btn.addEventListener('click', function () {
        const url = absoluteUrl(btn.getAttribute('data-url'));
        navigator.clipboard.writeText(url).then(function () {
          btn.textContent = '已复制';
          setTimeout(function () { btn.textContent = '复制 URL'; }, 1500);
        }).catch(function () {
          alert('复制失败: ' + url);
        });
      });
    });
    fileList.querySelectorAll('.btn-danger').forEach(function (btn) {
      btn.addEventListener('click', function () {
        deleteEntry(btn.getAttribute('data-name'), btn.getAttribute('data-type'));
      });
    });
  }

  async function deleteEntry(name, type) {
    if (currentPath) {
      alert('删除仅支持根目录');
      return;
    }
    if (!confirm('确定删除 "' + name + '"？')) return;
    const url = type === 'dir'
      ? '/api/files/dir/' + encodeURIComponent(name)
      : '/api/files/' + encodeURIComponent(name);
    try {
      const res = await fetch(url, { method: 'DELETE' });
      const data = await res.json();
      if (data.success) {
        loadList();
      } else {
        alert(data.error || '删除失败');
      }
    } catch {
      alert('网络错误');
    }
  }

  function uploadFile(file) {
    const form = new FormData();
    form.append('file', file);
    form.append('extract', 'true');

    const xhr = new XMLHttpRequest();
    progressWrap.hidden = false;
    progressBar.style.setProperty('--pct', '0%');
    progressText.textContent = '0%';
    setStatus('上传中: ' + file.name);

    xhr.upload.onprogress = function (e) {
      if (e.lengthComputable) {
        const pct = Math.round((e.loaded / e.total) * 100);
        progressBar.style.setProperty('--pct', pct + '%');
        progressText.textContent = pct + '%';
      }
    };

    xhr.onload = function () {
      progressWrap.hidden = true;
      try {
        const data = JSON.parse(xhr.responseText);
        if (xhr.status === 200 && data.success) {
          let msg = '上传成功: ' + data.filename;
          if (data.extracted_dir) msg += '（已解压至 ' + data.extracted_dir + '/）';
          if (data.error) msg += ' — ' + data.error;
          setStatus(msg, data.error ? 'err' : 'ok');
          loadList();
        } else {
          setStatus(data.error || '上传失败', 'err');
        }
      } catch {
        setStatus('上传失败', 'err');
      }
    };

    xhr.onerror = function () {
      progressWrap.hidden = true;
      setStatus('网络错误', 'err');
    };

    xhr.open('POST', '/api/files');
    xhr.send(form);
  }

  const dropZone = document.getElementById('drop-zone');
  const fileInput = document.getElementById('file-input');

  dropZone.addEventListener('dragover', function (e) {
    e.preventDefault();
    dropZone.classList.add('dragover');
  });
  dropZone.addEventListener('dragleave', function () {
    dropZone.classList.remove('dragover');
  });
  dropZone.addEventListener('drop', function (e) {
    e.preventDefault();
    dropZone.classList.remove('dragover');
    if (e.dataTransfer.files.length > 0) uploadFile(e.dataTransfer.files[0]);
  });

  fileInput.addEventListener('change', function () {
    if (fileInput.files.length > 0) uploadFile(fileInput.files[0]);
    fileInput.value = '';
  });

  document.getElementById('search-btn').addEventListener('click', function () {
    currentPrefix = prefixSearch.value.trim();
    loadList();
  });
  prefixSearch.addEventListener('keydown', function (e) {
    if (e.key === 'Enter') {
      currentPrefix = prefixSearch.value.trim();
      loadList();
    }
  });
  document.getElementById('refresh-btn').addEventListener('click', loadList);

  document.getElementById('logout-btn').addEventListener('click', async function () {
    try {
      await fetch('/api/ui/logout', { method: 'POST' });
    } finally {
      window.location.href = '/login.html';
    }
  });

  loadList();
})();
