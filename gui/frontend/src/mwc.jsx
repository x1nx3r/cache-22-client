import React from 'react';
import {createComponent} from '@lit/react';

import {MdFilledButton} from '@material/web/button/filled-button.js';
import {MdOutlinedButton} from '@material/web/button/outlined-button.js';
import {MdTextButton} from '@material/web/button/text-button.js';
import {MdFilledIconButton} from '@material/web/iconbutton/filled-icon-button.js';
import {MdFilledTonalIconButton} from '@material/web/iconbutton/filled-tonal-icon-button.js';
import {MdIconButton} from '@material/web/iconbutton/icon-button.js';
import {MdElevatedCard} from '@material/web/labs/card/elevated-card.js';
import {MdDialog} from '@material/web/dialog/dialog.js';
import {MdList} from '@material/web/list/list.js';
import {MdListItem} from '@material/web/list/list-item.js';
import {MdLinearProgress} from '@material/web/progress/linear-progress.js';
import {MdFilledSelect} from '@material/web/select/filled-select.js';
import {MdSelectOption} from '@material/web/select/select-option.js';
import {MdOutlinedTextField} from '@material/web/textfield/outlined-text-field.js';

export const FilledButton = createComponent({
    tagName: 'md-filled-button', elementClass: MdFilledButton, react: React,
    events: {onClick: 'click'},
});

export const OutlinedButton = createComponent({
    tagName: 'md-outlined-button', elementClass: MdOutlinedButton, react: React,
    events: {onClick: 'click'},
});

export const TextButton = createComponent({
    tagName: 'md-text-button', elementClass: MdTextButton, react: React,
    events: {onClick: 'click'},
});

export const FilledIconButton = createComponent({
    tagName: 'md-filled-icon-button', elementClass: MdFilledIconButton, react: React,
    events: {onClick: 'click'},
});

export const FilledTonalIconButton = createComponent({
    tagName: 'md-filled-tonal-icon-button', elementClass: MdFilledTonalIconButton, react: React,
    events: {onClick: 'click'},
});

export const IconButton = createComponent({
    tagName: 'md-icon-button', elementClass: MdIconButton, react: React,
    events: {onClick: 'click'},
});

export const ElevatedCard = createComponent({
    tagName: 'md-elevated-card', elementClass: MdElevatedCard, react: React,
});

export const Dialog = createComponent({
    tagName: 'md-dialog', elementClass: MdDialog, react: React,
    events: {onClose: 'close', onCancel: 'cancel'},
});

export const List = createComponent({
    tagName: 'md-list', elementClass: MdList, react: React,
});

export const ListItem = createComponent({
    tagName: 'md-list-item', elementClass: MdListItem, react: React,
    events: {onClick: 'click'},
});

export const LinearProgress = createComponent({
    tagName: 'md-linear-progress', elementClass: MdLinearProgress, react: React,
});

export const FilledSelect = createComponent({
    tagName: 'md-filled-select', elementClass: MdFilledSelect, react: React,
    events: {onChange: 'change', onInput: 'input'},
});

export const SelectOption = createComponent({
    tagName: 'md-select-option', elementClass: MdSelectOption, react: React,
});

export const OutlinedTextField = createComponent({
    tagName: 'md-outlined-text-field', elementClass: MdOutlinedTextField, react: React,
    events: {onInput: 'input', onChange: 'change'},
});
