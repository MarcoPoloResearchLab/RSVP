// @ts-check

const userMenu = document.querySelector('[data-user-menu]');
const userMenuSummary = document.querySelector('[data-user-menu] > summary');
const settingsDialog = document.querySelector('[data-settings-dialog]');
const settingsOpenButton = document.querySelector('[data-settings-open]');

if (!(userMenu instanceof HTMLDetailsElement) || !(userMenuSummary instanceof HTMLElement) || !(settingsDialog instanceof HTMLDialogElement) || !(settingsOpenButton instanceof HTMLButtonElement)) {
    throw new Error('The authenticated settings contract is incomplete.');
}

const settingsCloseButton = settingsDialog.querySelector('[data-settings-close]');
const settingsStatus = settingsDialog.querySelector('[data-settings-status]');
const rubricTabs = Array.from(settingsDialog.querySelectorAll('[data-settings-tab]'));
const rubricPanels = Array.from(settingsDialog.querySelectorAll('[data-settings-panel]'));
const settingsHashPrefix = '#settings/';
const defaultRubric = 'calendars-lanes';
const integrationsRubric = 'integrations';

if (!(settingsCloseButton instanceof HTMLButtonElement) || !(settingsStatus instanceof HTMLElement) || rubricTabs.length === 0 || rubricPanels.length === 0) {
    throw new Error('The settings dialog contract is incomplete.');
}

/** @param {string} rubric */
const setActiveRubric = (rubric) => {
    const selectedTab = rubricTabs.find((tab) => tab instanceof HTMLButtonElement && tab.dataset.settingsTab === rubric);
    const selectedPanel = rubricPanels.find((panel) => panel instanceof HTMLElement && panel.dataset.settingsPanel === rubric);
    if (!(selectedTab instanceof HTMLButtonElement) || !(selectedPanel instanceof HTMLElement)) {
        return false;
    }
    for (const tab of rubricTabs) {
        if (!(tab instanceof HTMLButtonElement)) {
            throw new TypeError('A settings rubric control is invalid.');
        }
        const selected = tab === selectedTab;
        tab.setAttribute('aria-selected', String(selected));
        tab.tabIndex = selected ? 0 : -1;
    }
    for (const panel of rubricPanels) {
        if (!(panel instanceof HTMLElement)) {
            throw new TypeError('A settings rubric panel is invalid.');
        }
        panel.hidden = panel !== selectedPanel;
    }
    return true;
};

const rubricFromHash = () => window.location.hash.startsWith(settingsHashPrefix) ? window.location.hash.slice(settingsHashPrefix.length) : '';

/** @param {string} rubric */
const openSettings = (rubric) => {
    const requestedRubric = setActiveRubric(rubric) ? rubric : defaultRubric;
    setActiveRubric(requestedRubric);
    userMenu.open = false;
    if (window.location.hash !== `${settingsHashPrefix}${requestedRubric}`) {
        window.history.replaceState(null, '', `${window.location.pathname}${window.location.search}${settingsHashPrefix}${requestedRubric}`);
    }
    if (!settingsDialog.open) {
        settingsDialog.showModal();
    }
    if (requestedRubric === integrationsRubric) {
        void loadCalendarSynchronizationStatus();
    }
};

const closeSettings = () => {
    if (settingsDialog.open) {
        settingsDialog.close();
    }
};

settingsOpenButton.addEventListener('click', () => openSettings(rubricFromHash() || defaultRubric));
settingsCloseButton.addEventListener('click', closeSettings);

settingsDialog.addEventListener('click', (event) => {
    if (event.target === settingsDialog) {
        closeSettings();
    }
});

settingsDialog.addEventListener('close', () => {
    if (window.location.hash.startsWith(settingsHashPrefix)) {
        window.history.replaceState(null, '', `${window.location.pathname}${window.location.search}`);
    }
    userMenuSummary.focus();
});

for (const [tabIndex, tab] of rubricTabs.entries()) {
    if (!(tab instanceof HTMLButtonElement) || !tab.dataset.settingsTab) {
        throw new TypeError('A settings rubric control is invalid.');
    }
    tab.addEventListener('click', () => openSettings(tab.dataset.settingsTab || defaultRubric));
    tab.addEventListener('keydown', (event) => {
        let nextIndex = tabIndex;
        if (event.key === 'ArrowDown' || event.key === 'ArrowRight') {
            nextIndex = (tabIndex + 1) % rubricTabs.length;
        } else if (event.key === 'ArrowUp' || event.key === 'ArrowLeft') {
            nextIndex = (tabIndex - 1 + rubricTabs.length) % rubricTabs.length;
        } else if (event.key === 'Home') {
            nextIndex = 0;
        } else if (event.key === 'End') {
            nextIndex = rubricTabs.length - 1;
        } else {
            return;
        }
        event.preventDefault();
        const nextTab = rubricTabs[nextIndex];
        if (nextTab instanceof HTMLButtonElement && nextTab.dataset.settingsTab) {
            openSettings(nextTab.dataset.settingsTab);
            nextTab.focus();
        }
    });
}

document.addEventListener('click', (event) => {
    if (userMenu.open && event.target instanceof Node && !userMenu.contains(event.target)) {
        userMenu.open = false;
    }
});

document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && userMenu.open && !settingsDialog.open) {
        userMenu.open = false;
        userMenuSummary.focus();
    }
});

window.addEventListener('hashchange', () => {
    const rubric = rubricFromHash();
    if (rubric) {
        openSettings(rubric);
    } else {
        closeSettings();
    }
});

const browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
for (const timezoneInput of settingsDialog.querySelectorAll('[data-settings-client-timezone]')) {
    if (!(timezoneInput instanceof HTMLInputElement)) {
        throw new TypeError('A settings timezone input is invalid.');
    }
    timezoneInput.value = browserTimezone;
}

const organizerTimezoneInput = settingsDialog.querySelector('[data-settings-organizer-timezone]');
const useBrowserTimezoneButton = settingsDialog.querySelector('[data-settings-use-browser-timezone]');
const timezoneOptions = settingsDialog.querySelector('[data-settings-timezone-options]');
if (!(organizerTimezoneInput instanceof HTMLInputElement) || !(useBrowserTimezoneButton instanceof HTMLButtonElement) || !(timezoneOptions instanceof HTMLDataListElement)) {
    throw new Error('The account timezone contract is incomplete.');
}
const supportedTimezones = typeof Intl.supportedValuesOf === 'function' ? Intl.supportedValuesOf('timeZone') : [];
for (const timezoneName of new Set(['UTC', ...supportedTimezones])) {
    const option = document.createElement('option');
    option.value = timezoneName;
    timezoneOptions.append(option);
}
useBrowserTimezoneButton.addEventListener('click', () => {
    organizerTimezoneInput.value = browserTimezone;
    organizerTimezoneInput.focus();
});

/** @param {string} message */
const showSettingsError = (message) => {
    settingsStatus.classList.remove('visually-hidden');
    settingsStatus.textContent = message;
};

/** @param {Response} response */
const requireSettingsResponse = async (response) => {
    if (response.ok) {
        return;
    }
    let message = `Settings operation failed with status ${response.status}.`;
    try {
        const body = await response.json();
        if (body && body.error && typeof body.error.message === 'string') {
            message = body.error.message;
        }
    } catch (_error) {
        // The HTTP status remains the canonical browser error.
    }
    throw new Error(message);
};

const synchronizationStatus = settingsDialog.querySelector('[data-settings-sync-status]');
const importTaskStateRow = settingsDialog.querySelector('[data-calendar-task-state-row]');
const importTaskState = settingsDialog.querySelector('[data-calendar-task-state]');
const importTaskError = settingsDialog.querySelector('[data-calendar-task-error]');
const synchronizationStateRow = settingsDialog.querySelector('[data-calendar-sync-state-row]');
const synchronizationState = settingsDialog.querySelector('[data-calendar-sync-state]');
const lastSuccessfulRow = settingsDialog.querySelector('[data-calendar-last-success-row]');
const lastSuccessfulTime = settingsDialog.querySelector('[data-calendar-last-success]');
const synchronizationError = settingsDialog.querySelector('[data-calendar-sync-error]');
let synchronizationStatusLoaded = false;
let synchronizationStatusLoading = false;
let calendarImportWasActive = false;

const loadCalendarSynchronizationStatus = async () => {
    if (synchronizationStatusLoaded || synchronizationStatusLoading || !(synchronizationStatus instanceof HTMLElement)) {
        return;
    }
    const statusURL = synchronizationStatus.dataset.statusUrl;
    if (!statusURL || !(importTaskStateRow instanceof HTMLElement) || !(importTaskState instanceof HTMLElement) || !(importTaskError instanceof HTMLElement) || !(synchronizationStateRow instanceof HTMLElement) || !(synchronizationState instanceof HTMLElement) || !(lastSuccessfulRow instanceof HTMLElement) || !(lastSuccessfulTime instanceof HTMLTimeElement) || !(synchronizationError instanceof HTMLElement)) {
        throw new Error('The calendar synchronization status contract is incomplete.');
    }
    synchronizationStatusLoading = true;
    try {
        const response = await fetch(statusURL, {credentials: 'same-origin', headers: {'Accept': 'application/json'}});
        await requireSettingsResponse(response);
        const body = await response.json();
        if (!body || !body.synchronization || typeof body.synchronization.state !== 'string' || typeof body.synchronization.error !== 'boolean' || typeof body.synchronization.last_successful_sync !== 'string') {
            throw new Error('The calendar synchronization status response is invalid.');
        }
        if (body.task !== undefined && (!body.task || typeof body.task.state !== 'string' || typeof body.task.active !== 'boolean' || typeof body.task.error !== 'boolean')) {
            throw new Error('The calendar import task response is invalid.');
        }
        const taskStateLabels = {pending: 'Queued', running: 'Running', succeeded: 'Complete', failed: body.task && body.task.active ? 'Retry scheduled' : 'Needs attention'};
        importTaskStateRow.hidden = !body.task;
        importTaskState.textContent = body.task ? taskStateLabels[body.task.state] || body.task.state : '';
        importTaskError.hidden = !body.task || !body.task.error;
        importTaskError.textContent = body.task && body.task.active
            ? 'The calendar import did not complete. RSVP will retry the task automatically.'
            : 'The calendar import needs attention. Review the application logs.';
        synchronizationStateRow.hidden = body.synchronization.state === '';
        synchronizationState.textContent = body.synchronization.state;
        lastSuccessfulRow.hidden = body.synchronization.last_successful_sync === '';
        lastSuccessfulTime.dateTime = body.synchronization.last_successful_sync;
        lastSuccessfulTime.textContent = body.synchronization.last_successful_sync;
        synchronizationError.hidden = !body.synchronization.error;
        const calendarImportCompleted = calendarImportWasActive && body.task && body.task.state === 'succeeded';
        calendarImportWasActive = Boolean(body.task && body.task.active);
        if (calendarImportCompleted) {
            window.location.reload();
            return;
        }
        synchronizationStatusLoaded = !body.task || !body.task.active;
        if (!synchronizationStatusLoaded) {
            window.setTimeout(() => void loadCalendarSynchronizationStatus(), 1000);
        }
    } catch (error) {
        showSettingsError(error instanceof Error ? error.message : 'Calendar synchronization status failed.');
    } finally {
        synchronizationStatusLoading = false;
    }
};

/** @param {HTMLFormElement} form */
const settingsFormPayload = (form) => {
    /** @type {Record<string, string|null>} */
    const payload = {};
    for (const [name, rawValue] of new FormData(form).entries()) {
        if (typeof rawValue !== 'string') {
            continue;
        }
        if (rawValue === '') {
            if (name === 'ends_at') {
                payload[name] = null;
            }
            continue;
        }
        payload[name] = name === 'ends_at' ? new Date(rawValue).toISOString() : rawValue;
    }
    return payload;
};

for (const kindSelect of settingsDialog.querySelectorAll('[data-settings-lane-kind]')) {
    if (!(kindSelect instanceof HTMLSelectElement)) {
        throw new TypeError('A settings lane kind control is invalid.');
    }
    const form = kindSelect.closest('form');
    const endInput = form ? form.querySelector('[data-settings-lane-end]') : null;
    if (!(endInput instanceof HTMLInputElement)) {
        throw new TypeError('A settings lane end control is invalid.');
    }
    const applyLaneKind = () => {
        const finite = kindSelect.value === 'finite';
        endInput.disabled = !finite;
        endInput.required = finite;
        if (!finite) {
            endInput.value = '';
        }
    };
    kindSelect.addEventListener('change', applyLaneKind);
    applyLaneKind();
}

for (const resourceForm of settingsDialog.querySelectorAll('[data-settings-resource-form]')) {
    if (!(resourceForm instanceof HTMLFormElement)) {
        throw new TypeError('A settings resource form is invalid.');
    }
    resourceForm.addEventListener('submit', async (event) => {
        event.preventDefault();
        const resourceURL = resourceForm.dataset.resourceUrl;
        const method = resourceForm.dataset.method;
        if (!resourceURL || !method) {
            throw new Error('A settings resource form contract is incomplete.');
        }
        const submitButton = resourceForm.querySelector('button[type="submit"]');
        if (submitButton instanceof HTMLButtonElement) {
            submitButton.disabled = true;
        }
        try {
            const response = await fetch(resourceURL, {
                method,
                credentials: 'same-origin',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify(settingsFormPayload(resourceForm)),
            });
            await requireSettingsResponse(response);
            window.location.reload();
        } catch (error) {
            showSettingsError(error instanceof Error ? error.message : 'Settings resource operation failed.');
            if (submitButton instanceof HTMLButtonElement) {
                submitButton.disabled = false;
            }
        }
    });
}

for (const actionButton of settingsDialog.querySelectorAll('[data-settings-resource-action]')) {
    if (!(actionButton instanceof HTMLButtonElement)) {
        throw new TypeError('A settings resource action is invalid.');
    }
    actionButton.addEventListener('click', async () => {
        const method = actionButton.dataset.settingsResourceAction;
        const resourceURL = actionButton.dataset.resourceUrl;
        if (!method || !resourceURL) {
            throw new Error('A settings resource action contract is incomplete.');
        }
        actionButton.disabled = true;
        try {
            /** @type {RequestInit} */
            const request = {method, credentials: 'same-origin'};
            if (method !== 'DELETE') {
                request.headers = {'Content-Type': 'application/json'};
                request.body = JSON.stringify({display_order: Number(actionButton.dataset.displayOrder)});
            }
            const response = await fetch(resourceURL, request);
            await requireSettingsResponse(response);
            window.location.reload();
        } catch (error) {
            showSettingsError(error instanceof Error ? error.message : 'Settings resource operation failed.');
            actionButton.disabled = false;
        }
    });
}

const authorizationButton = settingsDialog.querySelector('[data-settings-calendar-authorize]');
if (authorizationButton instanceof HTMLButtonElement) {
    authorizationButton.addEventListener('click', async () => {
        const resourceURL = authorizationButton.dataset.resourceUrl;
        if (!resourceURL) {
            throw new Error('The calendar authorization URL is absent.');
        }
        authorizationButton.disabled = true;
        try {
            const response = await fetch(resourceURL, {
                method: 'POST', credentials: 'same-origin',
                headers: {'Content-Type': 'application/json'}, body: '{}',
            });
            await requireSettingsResponse(response);
            const body = await response.json();
            if (!body || typeof body.authorization_url !== 'string') {
                throw new Error('The calendar authorization response is invalid.');
            }
            window.location.assign(body.authorization_url);
        } catch (error) {
            showSettingsError(error instanceof Error ? error.message : 'Calendar authorization failed.');
            authorizationButton.disabled = false;
        }
    });
}

const initialRubric = rubricFromHash();
if (initialRubric) {
    openSettings(initialRubric);
}
