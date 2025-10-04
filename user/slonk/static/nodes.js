document.addEventListener('DOMContentLoaded', function () {
    fetchNodes();
});

function fetchNodes() {
    const apiUrl = '/api/nodes'; 
    let nodesArray = [];

    fetch(apiUrl)
        .then(response => response.json())
        .then(data => {
            // Convert object to array and sort by node ID in descending order
            nodesArray = Object.entries(data).sort((a, b) => b[0] - a[0]);
            renderTable(nodesArray);
        })
        .catch(error => console.error('Error fetching data:', error));
}

function renderTable(nodesArray) {
    const tableBody = document.getElementById('nodesTable').getElementsByTagName('tbody')[0];
    tableBody.innerHTML = '';

    for (let i = 0; i < nodesArray.length; i++) {
        let row = tableBody.insertRow();
        let cell1 = row.insertCell(0);
        let cell2 = row.insertCell(1);
        let cell3 = row.insertCell(2);
        let cell4 = row.insertCell(3);

        cell1.innerHTML = nodesArray[i][1].metadata.name;

        try {
            const goalState = nodesArray[i][1]?.spec?.slurmNodeSpec?.goalState ?? "up"
            if (goalState === "down") {
                cell2.style.color = 'red';
            }
            cell2.innerHTML = `<pre>${JSON.stringify(nodesArray[i][1].spec, null, 2)}</pre>`;
        } catch (e) {
            cell2.innerHTML = `<pre>"UNKNOWN"</pre>`;
            cell2.style.color = 'red';
        }
        // cell2.innerHTML = `<pre>${JSON.stringify(nodesArray[i][1].spec, null, 2)}</pre>`;

        try {
            const removed = nodesArray[i][1]?.status?.k8sNodeStatus?.removed ?? false
            if (removed === true) {
                cell3.style.color = 'red';
            }
            cell3.innerHTML = `<pre>${JSON.stringify(nodesArray[i][1].status.k8sNodeStatus, null, 2)}</pre>`;
        } catch (e) {
            cell3.innerHTML = `<pre>"UNKNOWN"</pre>`;
            cell3.style.color = 'red';
        }
        // cell3.innerHTML = `<pre>${JSON.stringify(nodesArray[i][1].status.k8sNodeStatus, null, 2)}</pre>`;


        try {
            const removed = nodesArray[i][1]?.status?.slurmNodeStatus?.removed ?? false
            if (removed === true) {
                cell4.style.color = 'red';
            }
            cell4.innerHTML = `<pre>${JSON.stringify(nodesArray[i][1].status.slurmNodeStatus, null, 2)}</pre>`;
        } catch (e) {
            cell4.innerHTML = `<pre>"UNKNOWN"</pre>`;
            cell4.style.color = 'red';
        }
        // cell4.innerHTML = `<pre>${JSON.stringify(nodesArray[i][1].status.slurmNodeStatus, null, 2)}</pre>`;
    }
}
