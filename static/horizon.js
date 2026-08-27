// @ts-check

const horizonView = document.querySelector('[data-horizon-view]');

if (horizonView instanceof HTMLElement) {
    const viewport = horizonView.querySelector('[data-horizon-viewport]');
    const board = horizonView.querySelector('[data-horizon-board]');
    const status = horizonView.querySelector('[data-horizon-status]');
    const calendarToggles = Array.from(horizonView.querySelectorAll('[data-calendar-toggle]'));
    const markerTargets = Array.from(horizonView.querySelectorAll('[data-marker-id]'));
    const scaleSteps = [6, 10, 16, 24];
    let scaleIndex = 1;

    if (!(viewport instanceof HTMLElement) || !(board instanceof HTMLElement) || !(status instanceof HTMLElement)) {
        throw new Error('The horizon view contract is incomplete.');
    }

    board.style.setProperty('--window-days', horizonView.dataset.windowDays || '90');

    const browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    if (!browserTimezone) {
        throw new Error('The browser did not supply an IANA timezone.');
    }
    for (const timezoneInput of horizonView.querySelectorAll('[data-client-timezone]')) {
        if (!(timezoneInput instanceof HTMLInputElement)) {
            throw new TypeError('A timezone input is invalid.');
        }
        timezoneInput.value = browserTimezone;
    }

    /** @param {Response} response */
    const requireResourceResponse = async (response) => {
        if (response.ok) {
            return;
        }
        let message = `Resource operation failed with status ${response.status}.`;
        try {
            const body = await response.json();
            if (body && body.error && typeof body.error.message === 'string') {
                message = body.error.message;
            }
        } catch (_error) {
            // The status message remains the canonical browser error.
        }
        throw new Error(message);
    };

    /** @param {HTMLFormElement} form */
    const formPayload = (form) => {
        /** @type {Record<string, string|number>} */
        const payload = {};
        for (const [name, rawValue] of new FormData(form).entries()) {
            if (typeof rawValue !== 'string' || rawValue === '') {
                continue;
            }
            const organizerLocalTime = form.hasAttribute('data-ingestion-draft-form') && (name === 'starts_at' || name === 'ends_at' || name === 'next_probe_at');
            if (!organizerLocalTime && (name === 'starts_at' || name === 'ends_at' || name === 'next_probe_at' || name === 'reference_time')) {
                payload[name] = new Date(rawValue).toISOString();
            } else if (name.endsWith('_seconds')) {
                payload[name] = Number(rawValue);
            } else {
                payload[name] = rawValue;
            }
        }
        return payload;
    };

    for (const kindSelect of horizonView.querySelectorAll('[data-lane-kind]')) {
        if (!(kindSelect instanceof HTMLSelectElement)) {
            throw new TypeError('A lane kind control is invalid.');
        }
        const form = kindSelect.closest('form');
        const endInput = form ? form.querySelector('[data-lane-end]') : null;
        if (!(endInput instanceof HTMLInputElement)) {
            throw new TypeError('A lane end control is invalid.');
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

    for (const referenceInput of horizonView.querySelectorAll('[data-reference-time]')) {
        if (!(referenceInput instanceof HTMLInputElement)) { throw new TypeError('A reference time input is invalid.'); }
        referenceInput.value = new Date().toISOString();
    }
    const quickMode = horizonView.querySelector('[data-quick-mode]');
    if (quickMode instanceof HTMLSelectElement) {
        const applyQuickMode = () => {
            const eventMode = quickMode.value === 'dated_event';
            for (const field of horizonView.querySelectorAll('[data-event-field] input, [data-event-field] select')) { if (field instanceof HTMLInputElement || field instanceof HTMLSelectElement) { field.disabled = !eventMode; } }
            for (const field of horizonView.querySelectorAll('[data-open-field] input')) { if (field instanceof HTMLInputElement) { field.disabled = eventMode; } }
        };
        quickMode.addEventListener('change', applyQuickMode);
        applyQuickMode();
    }

    for (const confirmButton of horizonView.querySelectorAll('[data-draft-confirm]')) {
        if (!(confirmButton instanceof HTMLButtonElement)) { throw new TypeError('A draft confirmation control is invalid.'); }
        confirmButton.addEventListener('click', async () => {
            const resourceURL = confirmButton.dataset.resourceUrl;
            if (!resourceURL) { throw new Error('The draft confirmation URL is absent.'); }
            confirmButton.disabled = true;
            try {
                const response = await fetch(resourceURL, {method: 'POST', credentials: 'same-origin', headers: {'Content-Type': 'application/json', 'Idempotency-Key': crypto.randomUUID()}, body: '{}'});
                await requireResourceResponse(response);
                window.location.reload();
            } catch (error) {
                status.classList.remove('visually-hidden');
                status.textContent = error instanceof Error ? error.message : 'Draft confirmation failed.';
                confirmButton.disabled = false;
            }
        });
    }

    for (const resourceForm of horizonView.querySelectorAll('[data-resource-form]')) {
        if (!(resourceForm instanceof HTMLFormElement)) {
            throw new TypeError('A resource form is invalid.');
        }
        resourceForm.addEventListener('submit', async (event) => {
            event.preventDefault();
            const resourceURL = resourceForm.dataset.resourceUrl;
            const method = resourceForm.dataset.method;
            if (!resourceURL || !method) {
                throw new Error('A resource form contract is incomplete.');
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
                    body: JSON.stringify(formPayload(resourceForm)),
                });
                await requireResourceResponse(response);
                window.location.reload();
            } catch (error) {
                status.classList.remove('visually-hidden');
                status.textContent = error instanceof Error ? error.message : 'Resource operation failed.';
                if (submitButton instanceof HTMLButtonElement) {
                    submitButton.disabled = false;
                }
            }
        });
    }

    for (const actionButton of horizonView.querySelectorAll('[data-resource-action]')) {
        if (!(actionButton instanceof HTMLButtonElement)) {
            throw new TypeError('A resource action is invalid.');
        }
        actionButton.addEventListener('click', async () => {
            const method = actionButton.dataset.resourceAction;
            const resourceURL = actionButton.dataset.resourceUrl;
            if (!method || !resourceURL) {
                throw new Error('A resource action contract is incomplete.');
            }
            /** @type {Record<string, number|string>} */
            const payload = {};
            if (actionButton.dataset.displayOrder) {
                payload.display_order = Number(actionButton.dataset.displayOrder);
            }
            if (actionButton.hasAttribute('data-resolve-lane')) {
                payload.resolved_at = new Date().toISOString();
            }
            if (actionButton.hasAttribute('data-complete-probe')) {
                payload.state = 'completed';
            }
            actionButton.disabled = true;
            try {
                /** @type {RequestInit} */
                const request = {method, credentials: 'same-origin'};
                if (method !== 'DELETE') {
                    request.headers = {'Content-Type': 'application/json'};
                    request.body = JSON.stringify(payload);
                }
                const response = await fetch(resourceURL, request);
                await requireResourceResponse(response);
                window.location.reload();
            } catch (error) {
                status.classList.remove('visually-hidden');
                status.textContent = error instanceof Error ? error.message : 'Resource operation failed.';
                actionButton.disabled = false;
            }
        });
    }

    const authorizationButton = horizonView.querySelector('[data-calendar-authorize]');
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
                await requireResourceResponse(response);
                const body = await response.json();
                if (!body || typeof body.authorization_url !== 'string') {
                    throw new Error('The calendar authorization response is invalid.');
                }
                window.location.assign(body.authorization_url);
            } catch (error) {
                status.classList.remove('visually-hidden');
                status.textContent = error instanceof Error ? error.message : 'Calendar authorization failed.';
                authorizationButton.disabled = false;
            }
        });
    }

    const loadSourcesButton = horizonView.querySelector('[data-load-source-calendars]');
    const saveSourcesButton = horizonView.querySelector('[data-save-source-calendars]');
    const sourceList = horizonView.querySelector('[data-source-calendar-list]');
    if (loadSourcesButton instanceof HTMLButtonElement && saveSourcesButton instanceof HTMLButtonElement && sourceList instanceof HTMLElement) {
        const sourceURL = loadSourcesButton.dataset.sourceUrl;
        if (!sourceURL || sourceURL !== saveSourcesButton.dataset.sourceUrl) {
            throw new Error('The source calendar URL is invalid.');
        }
        loadSourcesButton.addEventListener('click', async () => {
            loadSourcesButton.disabled = true;
            try {
                const response = await fetch(sourceURL, {credentials: 'same-origin', headers: {'Accept': 'application/json'}});
                await requireResourceResponse(response);
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
                    input.dataset.sourceCalendar = '';
                    label.append(input, document.createTextNode(` ${source.name}`));
                    sourceList.append(label);
                }
                saveSourcesButton.hidden = false;
            } catch (error) {
                status.classList.remove('visually-hidden');
                status.textContent = error instanceof Error ? error.message : 'Source calendar listing failed.';
                loadSourcesButton.disabled = false;
            }
        });
        saveSourcesButton.addEventListener('click', async () => {
            saveSourcesButton.disabled = true;
            const providerCalendarIDs = Array.from(sourceList.querySelectorAll('[data-source-calendar]:checked')).map((input) => {
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
                await requireResourceResponse(response);
                window.location.reload();
            } catch (error) {
                status.classList.remove('visually-hidden');
                status.textContent = error instanceof Error ? error.message : 'Source calendar selection failed.';
                saveSourcesButton.disabled = false;
            }
        });
    }

    for (const syncButton of horizonView.querySelectorAll('[data-calendar-sync]')) {
        if (!(syncButton instanceof HTMLButtonElement)) {
            throw new TypeError('A calendar synchronization control is invalid.');
        }
        syncButton.addEventListener('click', async () => {
            const resourceURL = syncButton.dataset.resourceUrl;
            const mappingID = syncButton.dataset.mappingId;
            if (!resourceURL || !mappingID) {
                throw new Error('The calendar synchronization contract is incomplete.');
            }
            syncButton.disabled = true;
            try {
                const response = await fetch(resourceURL, {method: 'POST', credentials: 'same-origin', headers: {'Content-Type': 'application/json', 'Idempotency-Key': crypto.randomUUID()}, body: JSON.stringify({mapping_id: mappingID})});
                await requireResourceResponse(response);
                window.location.reload();
            } catch (error) {
                status.classList.remove('visually-hidden');
                status.textContent = error instanceof Error ? error.message : 'Calendar synchronization failed.';
                syncButton.disabled = false;
            }
        });
    }

    /** @param {string} token */
    const colorForToken = (token) => {
        let hash = 0;
        for (const character of token) {
            hash = ((hash << 5) - hash + character.charCodeAt(0)) | 0;
        }
        return `hsl(${Math.abs(hash) % 360} 52% 38%)`;
    };

    for (const colorElement of horizonView.querySelectorAll('[data-color-token]')) {
        if (!(colorElement instanceof HTMLElement) || !colorElement.dataset.colorToken) {
            throw new Error('A horizon calendar has no color token.');
        }
        colorElement.style.setProperty('--calendar-color', colorForToken(colorElement.dataset.colorToken));
    }

    /** @param {HTMLInputElement} toggle @param {boolean} visible */
    const applyCalendarVisibility = (toggle, visible) => {
        const calendarID = toggle.dataset.calendarToggle;
        if (!calendarID) {
            throw new Error('A calendar toggle has no calendar identifier.');
        }
        const calendar = horizonView.querySelector(`[data-calendar-id="${CSS.escape(calendarID)}"]`);
        if (!(calendar instanceof HTMLElement)) {
            throw new Error(`Calendar ${calendarID} is absent from the horizon view.`);
        }
        toggle.checked = visible;
        calendar.hidden = !visible;
    };

    for (const toggleElement of calendarToggles) {
        if (!(toggleElement instanceof HTMLInputElement)) {
            throw new TypeError('A calendar toggle is not an input.');
        }
        const calendarID = toggleElement.dataset.calendarToggle;
        if (!calendarID) {
            throw new Error('A calendar toggle has no calendar identifier.');
        }
        const visibilityURL = toggleElement.dataset.visibilityUrl;
        if (!visibilityURL) {
            throw new Error(`Calendar ${calendarID} has no visibility URL.`);
        }
        applyCalendarVisibility(toggleElement, toggleElement.checked);
        toggleElement.addEventListener('change', async () => {
            const requestedVisibility = toggleElement.checked;
            const previousVisibility = !requestedVisibility;
            toggleElement.disabled = true;
            try {
                const response = await fetch(visibilityURL, {
                    method: 'PATCH',
                    credentials: 'same-origin',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({visible: requestedVisibility}),
                });
				await requireResourceResponse(response);
                applyCalendarVisibility(toggleElement, requestedVisibility);
                status.textContent = `${requestedVisibility ? 'Showed' : 'Hid'} calendar ${calendarID}.`;
            } catch (error) {
                applyCalendarVisibility(toggleElement, previousVisibility);
                status.classList.remove('visually-hidden');
                status.textContent = error instanceof Error ? error.message : 'Calendar visibility update failed.';
            } finally {
                toggleElement.disabled = false;
            }
        });
    }

    /** @param {number} direction */
    const pan = (direction) => {
        viewport.scrollBy({left: direction * viewport.clientWidth * 0.6, behavior: 'smooth'});
        status.textContent = direction < 0 ? 'Panned backward.' : 'Panned forward.';
    };

    /** @param {number} direction */
    const scale = (direction) => {
        const previousWidth = board.scrollWidth;
        const viewportCenter = viewport.scrollLeft + viewport.clientWidth / 2;
        const centerRatio = previousWidth === 0 ? 0 : viewportCenter / previousWidth;
        scaleIndex = Math.max(0, Math.min(scaleSteps.length - 1, scaleIndex + direction));
        board.style.setProperty('--day-width', `${scaleSteps[scaleIndex]}px`);
        const nextCenter = centerRatio * board.scrollWidth;
        viewport.scrollLeft = Math.max(0, nextCenter - viewport.clientWidth / 2);
        status.textContent = direction < 0 ? 'Scaled out.' : 'Scaled in.';
    };

    for (const panButton of horizonView.querySelectorAll('[data-pan]')) {
        panButton.addEventListener('click', () => {
            if (!(panButton instanceof HTMLElement)) {
                return;
            }
            pan(panButton.dataset.pan === 'backward' ? -1 : 1);
        });
    }
    for (const scaleButton of horizonView.querySelectorAll('[data-scale]')) {
        scaleButton.addEventListener('click', () => {
            if (!(scaleButton instanceof HTMLElement)) {
                return;
            }
            scale(scaleButton.dataset.scale === 'out' ? -1 : 1);
        });
    }

    /** @param {HTMLElement} selectedMarker */
    const selectMarker = (selectedMarker) => {
        for (const markerElement of markerTargets) {
            if (!(markerElement instanceof HTMLElement)) {
                continue;
            }
            const isSelected = markerElement === selectedMarker;
            markerElement.classList.toggle('is-selected', isSelected);
            const markerID = markerElement.dataset.markerId;
            const details = markerID ? horizonView.querySelector(`[data-marker-details="${CSS.escape(markerID)}"]`) : null;
            if (details instanceof HTMLElement) {
                details.hidden = !isSelected;
                details.style.setProperty('--detail-position', markerElement.style.getPropertyValue('--marker-position'));
            }
        }
        selectedMarker.focus({preventScroll: true});
        selectedMarker.scrollIntoView({block: 'nearest', inline: 'center', behavior: 'smooth'});
        status.textContent = `Selected ${selectedMarker.getAttribute('aria-label') || 'marker'}`;
    };

    for (const markerElement of markerTargets) {
        if (!(markerElement instanceof HTMLElement)) {
            throw new TypeError('A horizon marker target is invalid.');
        }
        markerElement.addEventListener('focus', () => selectMarker(markerElement));
        markerElement.addEventListener('click', () => selectMarker(markerElement));
    }

    /** @param {number} direction */
    const selectRelativeMarker = (direction) => {
        const visibleMarkers = markerTargets.filter((markerElement) => markerElement instanceof HTMLElement && markerElement.offsetParent !== null);
        if (visibleMarkers.length === 0) {
            status.textContent = 'No visible markers are available.';
            return;
        }
        const activeIndex = visibleMarkers.indexOf(document.activeElement);
        const startIndex = activeIndex < 0 ? (direction > 0 ? -1 : 0) : activeIndex;
        const nextIndex = (startIndex + direction + visibleMarkers.length) % visibleMarkers.length;
        const nextMarker = visibleMarkers[nextIndex];
        if (nextMarker instanceof HTMLElement) {
            selectMarker(nextMarker);
        }
    };

    horizonView.addEventListener('keydown', (event) => {
        if (event.target instanceof HTMLElement && (event.target.isContentEditable || event.target.matches('input, textarea, select, button, a'))) {
            return;
        }
        const key = event.key.toLowerCase();
        if (key === 'h') {
            event.preventDefault();
            pan(-1);
        } else if (key === 'l') {
            event.preventDefault();
            pan(1);
        } else if (key === '-' || key === '_') {
            event.preventDefault();
            scale(-1);
        } else if (key === '+' || key === '=') {
            event.preventDefault();
            scale(1);
        } else if (key === 'j') {
            event.preventDefault();
            selectRelativeMarker(1);
        } else if (key === 'k') {
            event.preventDefault();
            selectRelativeMarker(-1);
        } else if (/^[1-9]$/.test(key)) {
            const toggle = calendarToggles[Number(key) - 1];
            if (toggle instanceof HTMLInputElement) {
                event.preventDefault();
                toggle.click();
            }
        }
    });
}
