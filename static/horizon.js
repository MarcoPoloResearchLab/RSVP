// @ts-check

const horizonSetup = document.querySelector('[data-horizon-setup]');

if (horizonSetup instanceof HTMLElement) {
    const form = horizonSetup.querySelector('[data-horizon-setup-form]');
    const status = horizonSetup.querySelector('[data-horizon-setup-status]');
    const timezoneInput = horizonSetup.querySelector('[data-client-timezone]');
    if (!(form instanceof HTMLFormElement) || !(status instanceof HTMLElement) || !(timezoneInput instanceof HTMLInputElement)) {
        throw new Error('The Horizon setup contract is incomplete.');
    }
    const browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    if (!browserTimezone) {
        throw new Error('The browser did not supply an IANA timezone.');
    }
    timezoneInput.value = browserTimezone;
    form.addEventListener('submit', async (event) => {
        event.preventDefault();
        const resourceURL = form.dataset.resourceUrl;
        if (!resourceURL) {
            throw new Error('The calendar resource URL is absent.');
        }
        const submitButton = form.querySelector('button[type="submit"]');
        if (!(submitButton instanceof HTMLButtonElement)) {
            throw new Error('The Horizon setup action is absent.');
        }
        submitButton.disabled = true;
        const payload = Object.fromEntries(new FormData(form).entries());
        try {
            const response = await fetch(resourceURL, {
                method: 'POST',
                credentials: 'same-origin',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify(payload),
            });
            if (!response.ok) {
                let message = `Horizon setup failed with status ${response.status}.`;
                try {
                    const body = await response.json();
                    if (body && body.error && typeof body.error.message === 'string') {
                        message = body.error.message;
                    }
                } catch (_error) {
                    // The status message remains the canonical browser error.
                }
                throw new Error(message);
            }
            window.location.assign('/horizon/');
        } catch (error) {
            status.classList.remove('visually-hidden');
            status.textContent = error instanceof Error ? error.message : 'Horizon setup failed.';
            submitButton.disabled = false;
        }
    });
}

const horizonView = document.querySelector('[data-horizon-view]');

if (horizonView instanceof HTMLElement) {
    const resourceRoot = horizonView;
    const viewport = horizonView.querySelector('[data-horizon-viewport]');
    const status = horizonView.querySelector('[data-horizon-status]');
    const calendarToggles = Array.from(horizonView.querySelectorAll('[data-calendar-toggle]'));
    const markerTargets = Array.from(horizonView.querySelectorAll('[data-marker-id]'));
    const scaleButtons = Array.from(horizonView.querySelectorAll('[data-scale-preset]'));

    if (!(viewport instanceof HTMLElement) || !(status instanceof HTMLElement)) {
        throw new Error('The horizon view contract is incomplete.');
    }

    const browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    if (!browserTimezone) {
        throw new Error('The browser did not supply an IANA timezone.');
    }
    for (const timezoneInput of resourceRoot.querySelectorAll('[data-client-timezone]')) {
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
        /** @type {Record<string, string|number|null>} */
        const payload = {};
        for (const [name, rawValue] of new FormData(form).entries()) {
            if (typeof rawValue !== 'string') {
                continue;
            }
            if (rawValue === '') {
                const control = form.elements.namedItem(name);
                if (control instanceof HTMLElement && control.hasAttribute('data-nullable-empty')) {
                    payload[name] = null;
                }
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

    for (const kindSelect of resourceRoot.querySelectorAll('[data-lane-kind]')) {
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

    for (const resourceForm of resourceRoot.querySelectorAll('[data-resource-form]')) {
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

    for (const actionButton of resourceRoot.querySelectorAll('[data-resource-action]')) {
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

    /** @param {string} value */
    const stableHash = (value) => {
        let hash = 0;
        for (const character of value) {
            hash = ((hash << 5) - hash + character.charCodeAt(0)) | 0;
        }
        return hash >>> 0;
    };

    const calendarControls = [...horizonView.querySelectorAll('[data-calendar-control]')];
    const assignCalendarPresentationColors = () => {
        const colorByCalendarID = new Map(calendarControls.map((control) => {
            if (!(control instanceof HTMLElement) || !control.dataset.calendarControl || !control.dataset.colorToken) {
                throw new Error('A horizon calendar control has incomplete presentation data.');
            }
            const toggle = control.querySelector('[data-calendar-toggle]');
            if (!(toggle instanceof HTMLInputElement)) {
                throw new Error(`Calendar ${control.dataset.calendarControl} has no visibility toggle.`);
            }
            const rank = stableHash(`${control.dataset.colorToken}\u0000${control.dataset.calendarControl}`);
            return [control.dataset.calendarControl, `hsl(${rank % 360} 64% 34%)`];
        }));
        for (const colorElement of horizonView.querySelectorAll('[data-color-token]')) {
            if (!(colorElement instanceof HTMLElement) || !colorElement.dataset.colorToken) {
                throw new Error('A horizon calendar has no color token.');
            }
            const calendarID = colorElement.dataset.calendarControl || colorElement.dataset.calendarId;
            const color = calendarID ? colorByCalendarID.get(calendarID) : undefined;
            if (!color) {
                throw new Error('A horizon calendar has no presentation color.');
            }
            colorElement.style.setProperty('--calendar-color', color);
        }
    };

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
    assignCalendarPresentationColors();

    /** @param {number} direction */
    const pan = (direction) => {
        const panButton = horizonView.querySelector(`[data-pan="${direction < 0 ? 'backward' : 'forward'}"]`);
        if (!(panButton instanceof HTMLButtonElement) || !panButton.dataset.navigationUrl) {
            throw new Error('A Horizon pan URL is absent.');
        }
        window.location.assign(panButton.dataset.navigationUrl);
    };

    /** @param {'day'|'week'|'month'|'year'} preset */
    const selectScale = (preset) => {
        const scaleButton = scaleButtons.find((candidate) => candidate instanceof HTMLButtonElement && candidate.dataset.scalePreset === preset);
        if (!(scaleButton instanceof HTMLButtonElement) || !scaleButton.dataset.navigationUrl) {
            throw new Error('A Horizon scale URL is absent.');
        }
        window.location.assign(scaleButton.dataset.navigationUrl);
    };

    for (const panButton of horizonView.querySelectorAll('[data-pan]')) {
        panButton.addEventListener('click', () => {
            if (!(panButton instanceof HTMLButtonElement)) {
                throw new TypeError('A Horizon pan control is invalid.');
            }
            pan(panButton.dataset.pan === 'backward' ? -1 : 1);
        });
    }
    for (const scaleButton of scaleButtons) {
        scaleButton.addEventListener('click', () => {
            if (!(scaleButton instanceof HTMLButtonElement)) {
                throw new TypeError('A Horizon scale control is invalid.');
            }
            const preset = scaleButton.dataset.scalePreset;
            if (preset !== 'day' && preset !== 'week' && preset !== 'month' && preset !== 'year') {
                throw new Error('A Horizon scale value is invalid.');
            }
            selectScale(preset);
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
        } else if (key === 'd' || key === 'w' || key === 'm' || key === 'y') {
            event.preventDefault();
            const preset = key === 'd' ? 'day' : key === 'w' ? 'week' : key === 'm' ? 'month' : 'year';
            selectScale(preset);
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
