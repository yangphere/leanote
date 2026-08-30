/* First-party BootstrapDialog compatibility source backed by Bootstrap 5. */
(function (window, $) {
    'use strict';
    var dialogs = {};
    var sequence = 0;
    function nextId() { sequence += 1; return 'leanote-dialog-' + sequence; }
    function resolveContent(value, dialog) {
        return typeof value === 'function' ? value.call(value, dialog) : value;
    }
    function text(value, nl2br) {
        value = value == null ? '' : String(value);
        return nl2br ? value.replace(/\r\n/g, '<br>').replace(/[\r\n]/g, '<br>') : value;
    }
    function appendContent(container, value, nl2br, dialog) {
        value = resolveContent(value, dialog);
        if (value && value.jquery) return container.append(value);
        if (value && value.nodeType) return container.append(value);
        return container.html(text(value, nl2br));
    }
    function Dialog(options) {
        this.options = $.extend(true, {
            id: nextId(), type: 'type-primary', size: 'size-normal', title: null,
            message: '', buttons: [], closable: true, closeByBackdrop: true,
            closeByKeyboard: true, nl2br: true, autodestroy: true, data: {},
            cssClass: '', onshow: null, onshown: null, onhide: null, onhidden: null
        }, options || {});
        this.indexedButtons = {};
        this.realized = false;
        this.opened = false;
        dialogs[this.options.id] = this;
    }
    Dialog.prototype.realize = function () {
        if (this.realized) return this;
        var o = this.options;
        dialogs[o.id] = this;
        var modal = $('<div class="modal fade" tabindex="-1" role="dialog" aria-hidden="true"></div>').attr('id', o.id);
        var dialog = $('<div class="modal-dialog"></div>');
        var content = $('<div class="modal-content"></div>');
        var header = $('<div class="modal-header"></div>');
        var title = $('<h5 class="modal-title"></h5>');
        appendContent(title, o.title == null ? this.getDefaultText() : o.title, o.nl2br, this);
        var message = $('<div class="bootstrap-dialog-message"></div>');
        appendContent(message, o.message, o.nl2br, this);
        var body = $('<div class="modal-body"></div>').append(message);
        var footer = $('<div class="modal-footer"></div>');
        if (o.closable) {
            $('<button type="button" class="btn-close" aria-label="Close"></button>').on('click', this.close.bind(this)).appendTo(header);
        }
        header.append(title);
        o.buttons.forEach(function (button, index) {
            var id = button.id || (o.id + '-button-' + index);
            var element = $('<button type="button" class="btn"></button>').attr('id', id).addClass(button.cssClass || 'btn-secondary').text(button.label || 'Confirm');
            element.on('click', function () { if (typeof button.action === 'function') button.action.call(element, this); }.bind(this));
            this.indexedButtons[id] = element;
            footer.append(element);
        }, this);
        var sizeClass = this.getSizeClass();
        if (sizeClass) dialog.addClass(sizeClass);
        if (o.cssClass) modal.addClass(o.cssClass);
        modal.addClass(o.type);
        content.append(header, body, footer); dialog.append(content); modal.append(dialog);
        $('body').append(modal);
        this.$modal = modal; this.$body = body; this.realized = true;
        modal.on('shown.bs.modal', function () {
            if (typeof o.onshown === 'function') o.onshown(this);
            if (this.closeRequested) {
                this.closeRequested = false;
                window.bootstrap.Modal.getOrCreateInstance(modal[0]).hide();
            }
        }.bind(this));
        modal.on('hide.bs.modal', function () { if (typeof o.onhide === 'function') o.onhide(this); }.bind(this));
        modal.on('hidden.bs.modal', function () {
            this.opened = false;
            this.closeRequested = false;
            if (typeof o.onhidden === 'function') o.onhidden(this);
            if (o.autodestroy) {
                delete dialogs[o.id];
                modal.remove();
                this.realized = false;
                this.$modal = null;
                this.$body = null;
                this.indexedButtons = {};
            }
        }.bind(this));
    };
    Dialog.prototype.getId = function () { return this.options.id; };
    Dialog.prototype.getDefaultText = function () {
        return ({ 'type-default': 'Tips', 'type-info': 'Tips', 'type-primary': 'Tips', 'type-success': 'Success', 'type-warning': 'Warning', 'type-danger': 'Danger' })[this.options.type] || 'Tips';
    };
    Dialog.prototype.getModalBody = function () { this.realize(); return this.$body; };
    Dialog.prototype.getModal = function () { this.realize(); return this.$modal; };
    Dialog.prototype.getModalDialog = function () { this.realize(); return this.$modal.find('.modal-dialog'); };
    Dialog.prototype.getModalContent = function () { this.realize(); return this.$modal.find('.modal-content'); };
    Dialog.prototype.getModalHeader = function () { this.realize(); return this.$modal.find('.modal-header'); };
    Dialog.prototype.getModalFooter = function () { this.realize(); return this.$modal.find('.modal-footer'); };
    Dialog.prototype.isRealized = function () { return this.realized; };
    Dialog.prototype.isOpened = function () { return this.opened; };
    Dialog.prototype.isAutodestroy = function () { return this.options.autodestroy; };
    Dialog.prototype.isClosable = function () { return this.options.closable; };
    Dialog.prototype.canCloseByBackdrop = function () { return this.options.closeByBackdrop; };
    Dialog.prototype.canCloseByKeyboard = function () { return this.options.closeByKeyboard; };
    Dialog.prototype.getTitle = function () { return this.options.title; };
    Dialog.prototype.setTitle = function (value) { this.options.title = value; if (this.realized) appendContent(this.$modal.find('.modal-title').empty(), value == null ? this.getDefaultText() : value, this.options.nl2br, this); return this; };
    Dialog.prototype.getMessage = function () { return this.options.message; };
    Dialog.prototype.setMessage = function (value) { this.options.message = value; if (this.realized) appendContent(this.$body.find('.bootstrap-dialog-message').empty(), value, this.options.nl2br, this); return this; };
    Dialog.prototype.getType = function () { return this.options.type; };
    Dialog.prototype.setType = function (value) { var previous = this.options.type; this.options.type = value; if (this.realized) this.$modal.removeClass(previous).addClass(value); return this; };
    Dialog.prototype.getSize = function () { return this.options.size; };
    Dialog.prototype.getSizeClass = function () { return ({ 'size-large': 'modal-lg', 'size-small': 'modal-sm', 'size-extra-large': 'modal-xl' })[this.options.size] || ''; };
    Dialog.prototype.setSize = function (value) { var previous = this.getSizeClass(); this.options.size = value; if (this.realized) { var next = this.getSizeClass(); this.$modal.find('.modal-dialog').removeClass(previous); if (next) this.$modal.find('.modal-dialog').addClass(next); } return this; };
    Dialog.prototype.getCssClass = function () { return this.options.cssClass; };
    Dialog.prototype.setCssClass = function (value) { var previous = this.options.cssClass; this.options.cssClass = value || ''; if (this.realized) { if (previous) this.$modal.removeClass(previous); if (this.options.cssClass) this.$modal.addClass(this.options.cssClass); } return this; };
    Dialog.prototype.syncModalOptions = function () {
        if (!this.realized || !window.bootstrap || !window.bootstrap.Modal) return this;
        var instance = window.bootstrap.Modal.getInstance(this.$modal[0]);
        if (!instance) return this;
        instance._config.backdrop = this.options.closable && this.options.closeByBackdrop ? true : 'static';
        instance._config.keyboard = this.options.closable && this.options.closeByKeyboard;
        return this;
    };
    Dialog.prototype.setClosable = function (value) { this.options.closable = Boolean(value); if (this.realized) this.$modal.find('.btn-close').toggle(this.options.closable); return this.syncModalOptions(); };
    Dialog.prototype.setCloseByBackdrop = function (value) { this.options.closeByBackdrop = Boolean(value); return this.syncModalOptions(); };
    Dialog.prototype.setCloseByKeyboard = function (value) { this.options.closeByKeyboard = Boolean(value); return this.syncModalOptions(); };
    Dialog.prototype.setAutodestroy = function (value) { this.options.autodestroy = Boolean(value); return this; };
    Dialog.prototype.setData = function (key, value) { this.options.data[key] = value; return this; };
    Dialog.prototype.getData = function (key) { return this.options.data[key]; };
    Dialog.prototype.getButtons = function () { return this.options.buttons; };
    Dialog.prototype.setButtons = function (buttons) { this.options.buttons = buttons || []; if (this.realized) { this.$modal.find('.modal-footer').empty(); this.indexedButtons = {}; this.options.buttons.forEach(this.addButtonElement.bind(this)); } return this; };
    Dialog.prototype.addButton = function (button) { this.options.buttons.push(button); if (this.realized) this.addButtonElement(button, this.options.buttons.length - 1); return this; };
    Dialog.prototype.addButtons = function (buttons) { (buttons || []).forEach(this.addButton.bind(this)); return this; };
    Dialog.prototype.addButtonElement = function (button, index) {
        var id = button.id || (this.options.id + '-button-' + index);
        var element = $('<button type="button" class="btn"></button>').attr('id', id).addClass(button.cssClass || 'btn-secondary').text(button.label || 'Confirm');
        element.on('click', function () { if (typeof button.action === 'function') button.action.call(element, this); }.bind(this));
        this.indexedButtons[id] = element;
        this.$modal.find('.modal-footer').append(element);
        return element;
    };
    Dialog.prototype.getButton = function (id) { return this.indexedButtons[id] || null; };
    Dialog.prototype.enableButtons = function (enabled) { Object.keys(this.indexedButtons).forEach(function (id) { this.indexedButtons[id].prop('disabled', !enabled); }, this); return this; };
    Dialog.prototype.onShow = function (callback) { this.options.onshow = callback; return this; };
    Dialog.prototype.onShown = function (callback) { this.options.onshown = callback; return this; };
    Dialog.prototype.onHide = function (callback) { this.options.onhide = callback; return this; };
    Dialog.prototype.onHidden = function (callback) { this.options.onhidden = callback; return this; };
    Dialog.prototype.open = function () {
        this.realize();
        this.closeRequested = false;
        if (typeof this.options.onshow === 'function') this.options.onshow(this);
        window.bootstrap.Modal.getOrCreateInstance(this.$modal[0], {
            backdrop: this.options.closable && this.options.closeByBackdrop ? true : 'static',
            keyboard: this.options.closable && this.options.closeByKeyboard
        }).show();
        this.opened = true; return this;
    };
    Dialog.prototype.show = Dialog.prototype.open;
    Dialog.prototype.close = function () {
        if (!this.realized) return this;
        var instance = window.bootstrap.Modal.getOrCreateInstance(this.$modal[0]);
        if (this.$modal.hasClass('show') || this.opened) {
            this.closeRequested = true;
            instance.hide();
        } else {
            instance.hide();
        }
        return this;
    };
    Dialog.show = function (options) { return new Dialog(options).open(); };
    Dialog.confirm = function (message, callback) { return Dialog.show({ title: 'Confirm?', message: message, closable: false, buttons: [{ label: 'Cancel', action: function (d) { if (callback) callback(false); d.close(); } }, { label: 'Confirm', cssClass: 'btn-primary', action: function (d) { if (callback) callback(true); d.close(); } }] }); };
    Dialog.alert = function (message, callback) { return Dialog.show({ message: message, closable: false, buttons: [{ label: 'Confirm', cssClass: 'btn-primary', action: function (d) { if (callback) callback(true); d.close(); } }] }); };
    Dialog.closeAll = function () { Object.keys(dialogs).forEach(function (id) { dialogs[id].close(); }); };
    Dialog.dialogs = dialogs;
    window.BootstrapDialog = Dialog;
}(window, window.jQuery));
