// @ts-check

const workspaceFrame = document.querySelector('[data-workspace-frame]');
const workspaceMessage = document.querySelector('[data-workspace-message]');
if (!(workspaceFrame instanceof HTMLIFrameElement) || !(workspaceMessage instanceof HTMLElement)) {
    throw new Error('The RSVP workspace contract is incomplete.');
}

const workspaceOrigin = new URL(workspaceFrame.dataset.workspaceOrigin || '', window.location.href);
if (!['http:', 'https:'].includes(workspaceOrigin.protocol) || workspaceOrigin.pathname !== '/') {
    throw new Error('The RSVP workspace origin is invalid.');
}
const workspacePaths = new Map([['horizon', '/horizon/'], ['events', '/events/'], ['venues', '/venues/']]);
let workspaceEnabled = false;

const showWorkspace = () => {
    if (!workspaceEnabled) return;
    const [workspace, ...section] = window.location.hash.slice(1).split('/');
    const path = workspacePaths.get(workspace) || '/horizon/';
    const destination = new URL(path, workspaceOrigin);
    if (section.length > 0) destination.hash = section.join('/');
    workspaceFrame.src = destination.href;
    workspaceFrame.hidden = false;
    workspaceMessage.hidden = true;
};

document.addEventListener('mpr-ui:auth:authenticated', () => {
    workspaceEnabled = true;
    showWorkspace();
});
document.addEventListener('mpr-ui:auth:unauthenticated', () => {
    workspaceEnabled = false;
    workspaceFrame.removeAttribute('src');
    workspaceFrame.hidden = true;
    workspaceMessage.hidden = false;
});
window.addEventListener('hashchange', showWorkspace);
