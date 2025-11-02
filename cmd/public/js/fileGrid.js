function deleteFileGrid(elem) {
    const filename = elem.parentElement.dataset.filename;

    if (confirm(`Are you sure you want to delete "${filename}"?`)) {
        deleteFileByName(filename);
    }
}

window.deleteFileGrid = deleteFileGrid;

export async function deleteFileByName(filename) {
    const formData = new FormData();
    formData.append('file_name', filename);

    const response = await fetch('/api/file/delete', {
        method: 'POST',
        body: formData
    });

    if (response.ok) {
        removeFileByName(filename);
        return true;
    } else {
        alert('Failed to delete file');
        return false;
    }
}

export function setVisibilityGrid(filename, isPublic) {
    const fileElements = document.getElementsByClassName('file-entry');

    for (const fileElement of fileElements) {
        if (fileElement.dataset.filename === filename) {
            // Update the data attribute
            fileElement.dataset.public = isPublic.toString();

            // Update the visibility icon and text in the grid
            const visibilityStatus = fileElement.querySelector('.visbility-status');
            if (visibilityStatus) {
                const icon = visibilityStatus.querySelector('use');
                const text = visibilityStatus.querySelector('span');

                if (isPublic) {
                    icon.href.baseVal = '/public/assets/feather-sprite.svg#unlock';
                    text.textContent = 'Public';
                } else {
                    icon.href.baseVal = '/public/assets/feather-sprite.svg#lock';
                    text.textContent = 'Private';
                }
            }
            break;
        }
    }
}

function removeFileByName(filename) {
    const fileElements = document.getElementsByClassName('file-entry');

    for (const fileElement of fileElements) {
        if (fileElement.dataset.filename === filename) {
            fileElement.remove();
            break;
        }
    }
}