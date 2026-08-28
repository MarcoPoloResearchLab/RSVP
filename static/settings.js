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

const browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
if (!browserTimezone) {
    throw new Error('The browser did not supply an IANA timezone.');
}
for (const timezoneInput of settingsDialog.querySelectorAll('[data-settings-client-timezone]')) {
    if (!(timezoneInput instanceof HTMLInputElement)) {
        throw new TypeError('A settings timezone input is invalid.');
    }
    timezoneInput.value = browserTimezone;
}

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
const synchronizationStateRow = settingsDialog.querySelector('[data-calendar-sync-state-row]');
const synchronizationState = settingsDialog.querySelector('[data-calendar-sync-state]');
const lastSuccessfulRow = settingsDialog.querySelector('[data-calendar-last-success-row]');
const lastSuccessfulTime = settingsDialog.querySelector('[data-calendar-last-success]');
const synchronizationError = settingsDialog.querySelector('[data-calendar-sync-error]');
let synchronizationStatusLoaded = false;
let synchronizationStatusLoading = false;

const loadCalendarSynchronizationStatus = async () => {
    if (synchronizationStatusLoaded || synchronizationStatusLoading || !(synchronizationStatus instanceof HTMLElement)) {
        return;
    }
    const statusURL = synchronizationStatus.dataset.statusUrl;
    if (!statusURL || !(synchronizationStateRow instanceof HTMLElement) || !(synchronizationState instanceof HTMLElement) || !(lastSuccessfulRow instanceof HTMLElement) || !(lastSuccessfulTime instanceof HTMLTimeElement) || !(synchronizationError instanceof HTMLElement)) {
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
        synchronizationStateRow.hidden = body.synchronization.state === '';
        synchronizationState.textContent = body.synchronization.state;
        lastSuccessfulRow.hidden = body.synchronization.last_successful_sync === '';
        lastSuccessfulTime.dateTime = body.synchronization.last_successful_sync;
        lastSuccessfulTime.textContent = body.synchronization.last_successful_sync;
        synchronizationError.hidden = !body.synchronization.error;
        synchronizationStatusLoaded = true;
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

const loadSourcesButton = settingsDialog.querySelector('[data-settings-load-source-calendars]');
const saveSourcesButton = settingsDialog.querySelector('[data-settings-save-source-calendars]');
const sourceList = settingsDialog.querySelector('[data-settings-source-calendar-list]');
if (loadSourcesButton instanceof HTMLButtonElement && saveSourcesButton instanceof HTMLButtonElement && sourceList instanceof HTMLElement) {
    const sourceURL = loadSourcesButton.dataset.sourceUrl;
    if (!sourceURL || sourceURL !== saveSourcesButton.dataset.sourceUrl) {
        throw new Error('The source calendar URL is invalid.');
    }
    loadSourcesButton.addEventListener('click', async () => {
        loadSourcesButton.disabled = true;
        try {
            const response = await fetch(sourceURL, {credentials: 'same-origin', headers: {'Accept': 'application/json'}});
            await requireSettingsResponse(response);
            const body = await response.json();
            if (!body || !Array.isArray(body.sources)) {
                throw new Error('The source calendar response is invalid.');
            }
            sourceList.replaceChildren();
            for (const source of body.sources) {
                if (!source || typeof source.id !== 'string' || typeof source.name !== 'string' || typeof source.selected !== 'boolean') {
                    throw new Error('A source calendar is invalid.');
                }
                const label = document.createElement('label');
                const input = document.createElement('input');
                input.type = 'checkbox';
                input.value = source.id;
                input.checked = source.selected;
                input.dataset.settingsSourceCalendar = '';
                label.append(input, document.createTextNode(source.name));
                sourceList.append(label);
            }
            saveSourcesButton.hidden = false;
        } catch (error) {
            showSettingsError(error instanceof Error ? error.message : 'Source calendar listing failed.');
            loadSourcesButton.disabled = false;
        }
    });
    saveSourcesButton.addEventListener('click', async () => {
        saveSourcesButton.disabled = true;
        const providerCalendarIDs = Array.from(sourceList.querySelectorAll('[data-settings-source-calendar]:checked')).map((input) => {
            if (!(input instanceof HTMLInputElement)) {
                throw new TypeError('A source calendar control is invalid.');
            }
            return input.value;
        });
        try {
            const response = await fetch(sourceURL, {
                method: 'PUT', credentials: 'same-origin', headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({provider_calendar_ids: providerCalendarIDs, timezone: browserTimezone}),
            });
            await requireSettingsResponse(response);
            window.location.reload();
        } catch (error) {
            showSettingsError(error instanceof Error ? error.message : 'Source calendar selection failed.');
            saveSourcesButton.disabled = false;
        }
    });
}

const initialRubric = rubricFromHash();
if (initialRubric) {
    openSettings(initialRubric);
}
