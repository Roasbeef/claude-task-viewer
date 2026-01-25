// Graph visualization using D3.js force-directed layout.

function initGraph(listID) {
    const container = document.getElementById('graph');
    const width = container.clientWidth;
    const height = container.clientHeight;

    // Create SVG.
    const svg = d3.select('#graph')
        .append('svg')
        .attr('width', width)
        .attr('height', height);

    // Add arrow marker for edges.
    svg.append('defs').append('marker')
        .attr('id', 'arrowhead')
        .attr('viewBox', '-0 -5 10 10')
        .attr('refX', 25)
        .attr('refY', 0)
        .attr('orient', 'auto')
        .attr('markerWidth', 8)
        .attr('markerHeight', 8)
        .append('path')
        .attr('d', 'M0,-5L10,0L0,5')
        .attr('fill', '#30363d');

    // Create container group for zooming.
    const g = svg.append('g');

    // Add zoom behavior.
    const zoom = d3.zoom()
        .scaleExtent([0.1, 4])
        .on('zoom', (event) => {
            g.attr('transform', event.transform);
        });
    svg.call(zoom);

    // Fetch graph data.
    fetch(`/api/lists/${listID}/graph`)
        .then(response => response.json())
        .then(data => renderGraph(g, data, width, height))
        .catch(err => {
            console.error('Failed to load graph:', err);
            container.innerHTML = '<p style="text-align:center;padding:2rem;color:#8b949e">Failed to load graph data</p>';
        });
}

function renderGraph(g, data, width, height) {
    if (data.nodes.length === 0) {
        d3.select('#graph')
            .html('<p style="text-align:center;padding:2rem;color:#8b949e">No tasks to display</p>');
        return;
    }

    // Create force simulation.
    const simulation = d3.forceSimulation(data.nodes)
        .force('link', d3.forceLink(data.edges)
            .id(d => d.id)
            .distance(120))
        .force('charge', d3.forceManyBody().strength(-400))
        .force('center', d3.forceCenter(width / 2, height / 2))
        .force('collision', d3.forceCollide().radius(40));

    // Draw edges.
    const edges = g.append('g')
        .selectAll('path')
        .data(data.edges)
        .enter()
        .append('path')
        .attr('class', 'graph-edge');

    // Draw nodes.
    const nodes = g.append('g')
        .selectAll('g')
        .data(data.nodes)
        .enter()
        .append('g')
        .attr('class', d => {
            let cls = `graph-node status-${d.status}`;
            if (d.isBlocked) cls += ' blocked';
            return cls;
        })
        .call(d3.drag()
            .on('start', dragStarted)
            .on('drag', dragged)
            .on('end', dragEnded));

    // Node circles.
    nodes.append('circle')
        .attr('r', 20);

    // Node labels (task ID).
    nodes.append('text')
        .text(d => `#${d.id}`)
        .attr('dy', '0.35em');

    // Tooltips.
    nodes.append('title')
        .text(d => `${d.label}\n${d.description || ''}`);

    // Click to navigate.
    nodes.on('click', (event, d) => {
        const listID = window.location.pathname.split('/')[2];
        window.location.href = `/lists/${listID}/tasks/${d.id}`;
    });

    // Update positions on tick.
    simulation.on('tick', () => {
        edges.attr('d', d => {
            const dx = d.target.x - d.source.x;
            const dy = d.target.y - d.source.y;
            return `M${d.source.x},${d.source.y}L${d.target.x},${d.target.y}`;
        });

        nodes.attr('transform', d => `translate(${d.x},${d.y})`);
    });

    // Drag functions.
    function dragStarted(event, d) {
        if (!event.active) simulation.alphaTarget(0.3).restart();
        d.fx = d.x;
        d.fy = d.y;
    }

    function dragged(event, d) {
        d.fx = event.x;
        d.fy = event.y;
    }

    function dragEnded(event, d) {
        if (!event.active) simulation.alphaTarget(0);
        d.fx = null;
        d.fy = null;
    }
}
