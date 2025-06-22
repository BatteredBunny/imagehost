const fileDropZone = document.getElementById('fileDropZone');
const fileInput = document.getElementById('file');
const fileName = document.getElementById('fileName');

fileDropZone.addEventListener('click', () => {
    fileInput.click();
});

fileInput.addEventListener('change', (e) => {
    const file = e.target.files[0];
    if (file) {
        fileName.textContent = file.name;
        fileDropZone.classList.add('file-selected');
    }
});

fileDropZone.addEventListener('dragover', (e) => {
    e.preventDefault();
    fileDropZone.classList.add('drag-over');
});

fileDropZone.addEventListener('dragleave', (e) => {
    e.preventDefault();
    fileDropZone.classList.remove('drag-over');
});

fileDropZone.addEventListener('drop', (e) => {
    e.preventDefault();
    fileDropZone.classList.remove('drag-over');

    const files = e.dataTransfer.files;
    if (files.length > 0) {
        fileInput.files = files;
        fileName.textContent = files[0].name;
        fileDropZone.classList.add('file-selected');
    }
});