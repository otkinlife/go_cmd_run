document.addEventListener('DOMContentLoaded', function() {
    // DOM elements
    const commandList = document.getElementById('commandList');
    const commandModal = document.getElementById('commandModal');
    const modalTitle = document.getElementById('modalTitle');
    const commandForm = document.getElementById('commandForm');
    const formFields = document.getElementById('formFields');
    const cmdNameInput = document.getElementById('cmdName');
    const outputDiv = document.getElementById('output');
    const closeBtn = document.querySelector('.close');

    let socket = null;

    // Fetch commands from API
    fetch('/api/commands')
        .then(response => response.json())
        .then(commands => {
            // Create buttons for each command
            for (const cmdName in commands) {
                const cmdBtn = document.createElement('button');
                cmdBtn.className = 'command-btn';
                cmdBtn.textContent = cmdName;
                cmdBtn.onclick = () => openCommandModal(cmdName, commands[cmdName]);
                commandList.appendChild(cmdBtn);
            }
        })
        .catch(error => {
            console.error('Error fetching commands:', error);
            outputDiv.textContent = 'Failed to load commands. Please check if the server is running.';
        });

    // Open command modal with form fields
    function openCommandModal(cmdName, cmdArgs) {
        // Set command name
        modalTitle.textContent = `Execute: ${cmdName}`;
        cmdNameInput.value = cmdName;

        // Clear previous form fields
        formFields.innerHTML = '';

        // Create form fields for each argument
        for (const argName in cmdArgs) {
            const argType = cmdArgs[argName];

            const formGroup = document.createElement('div');
            formGroup.className = 'form-group';

            const label = document.createElement('label');
            label.textContent = argName;
            label.htmlFor = `arg-${argName}`;

            const input = document.createElement('input');
            input.type = argType === 'int' ? 'number' : 'text';
            input.id = `arg-${argName}`;
            input.name = argName;
            input.required = true;

            formGroup.appendChild(label);
            formGroup.appendChild(input);
            formFields.appendChild(formGroup);
        }

        // Show modal
        commandModal.style.display = 'block';
    }

    // Close modal when clicking the X
    closeBtn.onclick = function() {
        commandModal.style.display = 'none';
    }

    // Close modal when clicking outside
    window.onclick = function(event) {
        if (event.target === commandModal) {
            commandModal.style.display = 'none';
        }
    }

    // Handle form submission
    commandForm.onsubmit = function(e) {
        e.preventDefault();

        // Clear previous output
        outputDiv.textContent = '';

        // Get form data
        const formData = new FormData(commandForm);
        const cmdName = formData.get('cmd');

        // Create args object for WebSocket
        const args = {};
        formData.forEach((value, key) => {
            if (key !== 'cmd') {
                args[key] = value;
            }
        });

        // Close modal
        commandModal.style.display = 'none';

        // Close previous WebSocket if exists
        if (socket && socket.readyState === WebSocket.OPEN) {
            socket.close();
        }

        // Create WebSocket connection
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws/execute`;
        socket = new WebSocket(wsUrl);

        socket.onopen = function() {
            // Send command request
            socket.send(JSON.stringify({
                cmd: cmdName,
                args: args
            }));
        };

        socket.onmessage = function(event) {
            // Append output to the output div
            outputDiv.textContent += event.data;
            // Auto-scroll to bottom
            outputDiv.scrollTop = outputDiv.scrollHeight;
        };

        socket.onerror = function(error) {
            console.error('WebSocket error:', error);
            outputDiv.textContent += '\nError: WebSocket connection failed';
        };

        socket.onclose = function() {
            console.log('WebSocket connection closed');
        };
    };
});
