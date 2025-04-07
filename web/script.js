document.getElementById('runOnce').addEventListener('change', () => {
    document.getElementById('intervalSeconds').disabled = true;
});
document.getElementById('runInterval').addEventListener('change', () => {
    document.getElementById('intervalSeconds').disabled = false;
});

document.getElementById('startForm').addEventListener('submit', async (e) => {
    e.preventDefault();

    const excelFileName = document.getElementById('excelFileName').value;
    const postgresUrl = document.getElementById('postgresUrl').value;
    const ignorantSheets = document.getElementById('ignorantSheets').value;
    const runOnce = document.getElementById('runOnce').checked;
    const intervalSeconds = runOnce ? 0 : parseInt(document.getElementById('intervalSeconds').value);

    const requestData = {
        excel_file_name: excelFileName,
        postgres_url: postgresUrl,
        ignorant_sheets: ignorantSheets.split(',').map(sheet => sheet.trim()),
        once: runOnce,
        interval_seconds: intervalSeconds
    };

    try {
        const response = await fetch('http://localhost:8080/api/start', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(requestData),
        });
        if (response.ok) updateInstances();
    } catch (err) {
        alert(`Error: ${err.message}`);
    }
});


async function updateInstances() {
    try {
        const response = await fetch('http://localhost:8080/api/instances');
        const instances = await response.json();
        const tbody = document.getElementById('instancesBody');
        tbody.innerHTML = '';

        instances.forEach(inst => {
            const row = document.createElement('tr');
            row.innerHTML = `
                <td>${inst.id}</td>
                <td>${inst.status}</td>
                <td><pre>${inst.config}</pre></td>
                <td>${new Date(inst.started_at).toLocaleString()}</td>
                <td>
                    ${inst.status === 'running' ? `<button onclick="stopInstance('${inst.id}')">Stop</button>` : `<button onclick="restartInstance('${inst.id}')">Restart</button>`}
                    <button onclick="deleteInstance('${inst.id}')">Delete</button>
                </td>
            `;
            tbody.appendChild(row);
        });
    } catch (err) {
        console.error('Error updating instances:', err);
    }
}

async function stopInstance(id) {
    try {
        const response = await fetch(`http://localhost:8080/api/stop/${id}`, {
            method: 'POST',
        });
        if (response.ok) updateInstances();
    } catch (err) {
        alert(`Error stopping instance: ${err.message}`);
    }
}

async function restartInstance(id) {
    try {
        const response = await fetch(`http://localhost:8080/api/restart/${id}`, {
            method: 'POST',
        });
        if (response.ok) updateInstances();
    } catch (err) {
        alert(`Error restarting instance: ${err.message}`);
    }
}

async function deleteInstance(id) {
    try {
        const response = await fetch(`http://localhost:8080/api/delete/${id}`, {
            method: 'POST',
        });
        if (response.ok) updateInstances();
    } catch (err) {
        alert(`Error deleting instance: ${err.message}`);
    }
}

setInterval(updateInstances, 5000);
updateInstances();