const deleteButtons = document.querySelectorAll('.delete-button');

for (let button of deleteButtons) {
    if (!button.dataset.confirm) {
        continue;
    }

    button.addEventListener('click', function(e) {
        const confirmMessage = e.target.dataset.confirm;

        if (!confirm(confirmMessage)) {
            e.preventDefault();
        }
    });
}