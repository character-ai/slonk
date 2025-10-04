document.addEventListener('DOMContentLoaded', function() {
    const apiUrl = '/api/jobs/active';
    let currentPage = 1;
    let recordsPerPage = 100;
    let jobsArray = [];
    let groupedJobs = {};
    let currentUserName = '';
    let userNodeCounts = {};

    const fetchData = () => {
        fetch(apiUrl)
            .then(response => {
                if (!response.ok) {
                    throw new Error(`HTTP error! status: ${response.status}`);
                }
                return response.json();
            })
            .then(data => {
                jobsArray = Object.entries(data).sort((a, b) => b[0] - a[0]);
                groupJobsByUserName();
                countUserNodes();
                createUserNameNavigation();
                if (Object.keys(groupedJobs).length > 0) {
                    // currentUserName = Object.keys(groupedJobs)[0];
                    changePage(1);
                }
            })
            .catch(error => {
                console.error('Error fetching data:', error);
                document.getElementById('errorMessage').textContent = 'Failed to fetch data. Please check your connection and try again.';
            });
    };

    function groupJobsByUserName() {
        groupedJobs = {};
        jobsArray.forEach(job => {
            const userName = job[1].spec.userName || 'Unknown';
            if (!groupedJobs[userName]) {
                groupedJobs[userName] = [];
            }
            groupedJobs[userName].push(job);
        });
    }

    function countUserNodes() {
        max = 0;
        userNodeCounts = {};
        Object.entries(groupedJobs).forEach(([userName, jobs]) => {
            userNodeCounts[userName] = jobs.reduce((total, job) => {
                const nodeCount = Object.keys(job[1].status.slurmJobRunCurrentStatus.physicalNodeSnapshots || {}).length;
                return total + nodeCount;
            }, 0);
            if (userNodeCounts[userName] > max) {
                max = userNodeCounts[userName];
                currentUserName = userName;
            }
        });
    }

    function createUserNameNavigation() {
        const nav = document.createElement('nav');
        nav.id = 'userNameNav';

        // Sort users by node count
        const sortedUsers = Object.entries(userNodeCounts)
            .sort((a, b) => b[1] - a[1])
            .map(entry => entry[0]);

        sortedUsers.forEach(userName => {
            const button = document.createElement('button');
            button.textContent = `${userName} (${userNodeCounts[userName]} nodes)`;
            button.onclick = () => {
                currentUserName = userName;
                currentPage = 1;
                changePage(1);
            };
            nav.appendChild(button);
        });
        document.body.insertBefore(nav, document.getElementById('pagination'));
    }

    window.previousPage = function() {
        if (currentPage > 1) {
            currentPage--;
            changePage(currentPage);
        }
    };

    window.nextPage = function() {
        let numPages = Math.ceil(groupedJobs[currentUserName].length / recordsPerPage);
        if (currentPage < numPages) {
            currentPage++;
            changePage(currentPage);
        }
    };

    function changePage(page) {
        const tableBody = document.getElementById('jobsTable').getElementsByTagName('tbody')[0];
        tableBody.innerHTML = '';

        let userJobs = groupedJobs[currentUserName];
        let numPages = Math.ceil(userJobs.length / recordsPerPage);
        let start = (page - 1) * recordsPerPage;
        let end = start + recordsPerPage;
        let paginatedItems = userJobs.slice(start, end);

        for (let i = 0; i < paginatedItems.length; i++) {
            let row = tableBody.insertRow();
            let cell1 = row.insertCell(0);
            let cell2 = row.insertCell(1);
            let cell3 = row.insertCell(2);
            let cell4 = row.insertCell(3);
            let cell5 = row.insertCell(4);

            cell1.innerHTML = paginatedItems[i][0];
            try {
                const state = paginatedItems[i][1]?.status?.slurmJobRunCurrentStatus?.state ?? "REMOVED"
                if (state === "REMOVED") {
                    cell2.style.color = 'red';
                } else if (state === "RUNNING") {
                    cell2.style.color = 'green';
                }

                cell2.innerHTML = `<pre>${JSON.stringify(state, null, 2)}</pre>`;
            } catch (e) {
                cell2.innerHTML = `<pre>"REMOVED"</pre>`;
                cell2.style.color = 'red';
            }
            cell3.innerHTML = `<pre>${JSON.stringify(paginatedItems[i][1].spec, null, 2)}</pre>`;

            createCollapsibleNodeList(cell4, JSON.stringify(paginatedItems[i][1].status.slurmJobRunCurrentStatus.physicalNodeSnapshots, null, 2));
            createCollapsibleContent(cell5, `<pre>${JSON.stringify(paginatedItems[i][1].status.slurmJobRunStatusHistory, null, 2)}</pre>`);
        }

        let pageInfo = document.getElementById('pageInfo');
        pageInfo.innerHTML = `${currentUserName} (${userNodeCounts[currentUserName]} nodes): Page ${page} of ${numPages}`;

        document.querySelector("#pagination button:first-child").disabled = currentPage === 1;
        document.querySelector("#pagination button:last-child").disabled = currentPage === numPages;
    }

    function createCollapsibleNodeList(cell, content) {
        let collapsible = document.createElement('div');
        collapsible.className = 'node-list';
    
        try {
            let jsonData = JSON.parse(content);
    
            if (jsonData && Object.keys(jsonData).length > 0) {
                Object.values(jsonData).forEach(node => {
                    let nodeDiv = document.createElement('div');
                    nodeDiv.className = 'node-item';
    
                    nodeDiv.innerHTML = `
                        <span class="node-name">${node.slurmNodeName}</span>
                        <span class="separator">|</span>
                        <span class="node-physical-name">${node.physicalNodeName}</span>
                        <span class="separator">|</span>
                        <span class="rank-container">
                            ${Array.from({length: 8}, (_, i) => `
                                <a href="api/proxy/${node.slurmNodeName}:${3724 + i}/" class="rank-link">${i}</a>
                            `).join('')}
                        </span>
                    `;
    
                    collapsible.appendChild(nodeDiv);
                });
            } else {
                collapsible.innerHTML = '<p>No data available</p>';
            }
        } catch (e) {
            collapsible.innerHTML = `<p>N/A</p>`;
        }
    
        cell.appendChild(collapsible);
        cell.className = 'collapsible open';
    
        cell.onclick = function() {
            cell.classList.toggle('open');
            cell.classList.toggle('closed');
        };
    }
    
    function createCollapsibleContent(cell, content) {
        let collapsible = document.createElement('div');
        collapsible.innerHTML = content;
        collapsible.style.display = 'none'; // Start hidden
        collapsible.className = 'collapsible closed'; // Start as closed
        cell.appendChild(collapsible);
        cell.className = 'collapsible closed'; // Apply the closed class to the cell
        cell.onclick = function() {
            let isOpen = collapsible.style.display === 'block';
            collapsible.style.display = isOpen ? 'none' : 'block';
            if (isOpen) {
                collapsible.className = 'collapsible closed';
                cell.className = 'collapsible closed';
            } else {
                collapsible.className = 'collapsible open';
                cell.className = 'collapsible open';
            }
        };
    }

    fetchData(); // Fetch and display data
});