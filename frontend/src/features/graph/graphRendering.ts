import type { Core, CollectionReturnValue } from 'cytoscape';

/** Probe before constructing Cytoscape, so machines without WebGL retain Canvas. */
export function supportsGraphWebGL(createCanvas = () => document.createElement('canvas')): boolean {
  try {
    const gl = createCanvas().getContext('webgl2');
    if (!gl) return false;
    const supported = !gl.isContextLost() && gl.getParameter(gl.MAX_TEXTURE_SIZE) >= 2048;
    gl.getExtension('WEBGL_lose_context')?.loseContext();
    return supported;
  } catch {
    return false;
  }
}

/** Keep only the previous highlighted neighborhood; hovering does not restyle the entire graph. */
export function createGraphFocus(cy: Core) {
  let previous: CollectionReturnValue = cy.collection();
  let lastSelected: string | null | undefined;
  let lastHovered: string | null | undefined;
  let hadFocus = false;
  return (selected: string | null, hovered: string | null) => {
    if (selected === lastSelected && hovered === lastHovered) return;
    lastSelected = selected; lastHovered = hovered;
    const focus = cy.getElementById(hovered ?? selected ?? '');
    const hasFocus = focus.length > 0;
    cy.batch(() => {
      previous.removeClass('is-selected is-hovered is-neighbor is-path');
      if (hasFocus !== hadFocus) cy.elements().toggleClass('is-dimmed', hasFocus);
      else if (hasFocus) previous.addClass('is-dimmed');
      const neighbors = focus.neighborhood();
      neighbors.nodes().addClass('is-neighbor');
      neighbors.edges().addClass('is-path');
      const selectedNode = cy.getElementById(selected ?? '');
      const hoveredNode = cy.getElementById(hovered ?? '');
      selectedNode.addClass('is-selected');
      hoveredNode.addClass('is-hovered');
      previous = focus.union(neighbors).union(selectedNode).union(hoveredNode);
      previous.removeClass('is-dimmed');
      hadFocus = hasFocus;
    });
  };
}
