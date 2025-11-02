import { formatTimeDate, relativeTime, humanizeBytes, mimeIsImage, mimeIsVideo, mimeIsAudio } from './utils.js';

function createFileEntry(file) {
    const template = document.getElementById('file-entry-template');
    const entry = template.content.cloneNode(true).querySelector('.file-entry');

    // Used by file modal
    entry.dataset.filename = file.FileName;
    entry.dataset.originalfilename = file.OriginalFileName || '';
    entry.dataset.filesize = humanizeBytes(file.FileSize);
    entry.dataset.filesizebytes = file.FileSize;
    entry.dataset.isimage = mimeIsImage(file.MimeType);
    entry.dataset.isvideo = mimeIsVideo(file.MimeType);
    entry.dataset.isaudio = mimeIsAudio(file.MimeType);
    entry.dataset.views = file.ViewsCount || 0;
    entry.dataset.public = file.Public;

    if (file.ExpiryDate !== "0001-01-01T00:00:00Z") {
        entry.dataset.expirydate = formatTimeDate(file.ExpiryDate);
        entry.dataset.expiryrelative = relativeTime(file.ExpiryDate);
    } else {
        entry.dataset.expirydate = '';
        entry.dataset.expiryrelative = '';
    }

    entry.dataset.createdat = formatTimeDate(file.CreatedAt);
    entry.dataset.createdatrelative = relativeTime(file.CreatedAt);

    entry.querySelector('.file-name').textContent = file.OriginalFileName || file.FileName;

    if (mimeIsImage(file.MimeType)) {
        const img = entry.querySelector('.preview-image');
        img.src = `/${file.FileName}`;
        img.style.display = 'block';
    } else if (mimeIsVideo(file.MimeType)) {
        const video = entry.querySelector('.preview-video');
        video.src = `/${file.FileName}`;
        video.style.display = 'block';
    } else if (mimeIsAudio(file.MimeType)) {
        const audio = entry.querySelector('.preview-audio');
        audio.style.display = 'flex';
    } else {
        const generic = entry.querySelector('.preview-generic');
        generic.style.display = 'flex';
    }

    if (file.ExpiryDate !== "0001-01-01T00:00:00Z") {
        const expiresInfo = entry.querySelector('.expires-info');
        expiresInfo.style.display = 'flex';
        expiresInfo.title = formatTimeDate(file.ExpiryDate);
        expiresInfo.querySelector('.expiry-text').textContent = relativeTime(file.ExpiryDate);
    }

    const viewsText = file.ViewsCount === 1 ? '1 view' : `${file.ViewsCount || 0} views`;
    entry.querySelector('.views-text').textContent = viewsText;

    const visibilityIcon = entry.querySelector('.visibility-icon');
    const visibilityText = entry.querySelector('.visibility-text');
    if (file.Public) {
        visibilityIcon.href.baseVal = '/public/assets/feather-sprite.svg#unlock';
        visibilityText.textContent = 'Public';
    } else {
        visibilityIcon.href.baseVal = '/public/assets/feather-sprite.svg#lock';
        visibilityText.textContent = 'Private';
    }

    return entry;
}

let currentPage = 0;
const filesPerPage = 8;
let totalFiles = 0;
let isLoading = false;

export function decrementTotalFiles() {
    totalFiles--;
}

export async function loadFiles(skip = 0) {
    if (isLoading) return;

    isLoading = true;

    const fileGrid = document.querySelector('.file-grid');
    const loadingOverlay = document.getElementById('file-grid-loading-overlay');
    const existingFiles = fileGrid.querySelectorAll('.file-entry');

    if (existingFiles.length > 0) {
        fileGrid.classList.add('loading');
        loadingOverlay.classList.add('visible');
    } else {
        setLoadingText("Loading...");
    }

    try {
        const response = await fetch(`/api/account/files?skip=${skip}`, {
            method: 'GET',
        });

        if (!response.ok) {
            fileGrid.classList.remove('loading');
            loadingOverlay.classList.remove('visible');
            setLoadingText("Failed to fetch files.");
            isLoading = false;
            return;
        }

        const data = await response.json();
        totalFiles = data.count || 0;

        existingFiles.forEach(file => file.remove());

        fileGrid.classList.remove('loading');
        loadingOverlay.classList.remove('visible');

        if (data.files && data.files.length > 0) {
            setLoadingText("");

            for (let file of data.files) {
                const fileEntry = createFileEntry(file);
                fileGrid.appendChild(fileEntry);
            }

            updatePaginationControls(skip);
        } else {
            setLoadingText("No files uploaded yet.");
            hidePaginationControls();
        }
    } catch (error) {
        console.error('Error loading files:', error);
        fileGrid.classList.remove('loading');
        loadingOverlay.classList.remove('visible');
        setLoadingText("Failed to load files.");
    } finally {
        isLoading = false;
    }
}

function updatePaginationControls(skip) {
    const paginationControls = document.getElementById('pagination-controls');
    const prevBtn = document.getElementById('prev-page-btn');
    const nextBtn = document.getElementById('next-page-btn');
    const pageInfo = document.getElementById('page-info');

    currentPage = Math.floor(skip / filesPerPage);
    const totalPages = Math.ceil(totalFiles / filesPerPage);

    if (totalPages <= 1) {
        hidePaginationControls();
        return;
    }

    paginationControls.style.display = 'flex';

    const startFile = skip + 1;
    const endFile = Math.min(skip + filesPerPage, totalFiles);
    pageInfo.textContent = `${startFile}-${endFile} of ${totalFiles}`;

    prevBtn.disabled = currentPage === 0;
    nextBtn.disabled = (currentPage + 1) >= totalPages;
}

function hidePaginationControls() {
    const paginationControls = document.getElementById('pagination-controls');
    paginationControls.style.display = 'none';
}

function nextPage() {
    const skip = (currentPage + 1) * filesPerPage;
    loadFiles(skip);
}

window.nextPage = nextPage;

function prevPage() {
    if (currentPage > 0) {
        const skip = (currentPage - 1) * filesPerPage;
        loadFiles(skip);
    }
}

window.prevPage = prevPage;

export function reloadCurrentPage() {
    const totalPages = Math.ceil(totalFiles / filesPerPage);

    if (currentPage >= totalPages) {
        currentPage = Math.max(0, totalPages - 1);
    }

    let skip = currentPage * filesPerPage;
    loadFiles(skip);
    updatePaginationControls(skip);
}

export function getCurrentPage() {
    return currentPage;
}

export function getFilesPerPage() {
    return filesPerPage;
}

function setLoadingText(text) {
    const loadingText = document.getElementById('file-grid-loading-text');
    if (text == "") {
        loadingText.innerText = "";
        loadingText.classList.add('hidden');
    } else {
        loadingText.innerText = text;
        loadingText.classList.remove('hidden');
    }
}

document.addEventListener('DOMContentLoaded', loadFiles());
