import React, {useState, useEffect, useRef, useCallback} from 'react';
import {ServerService, LibraryService, DownloadService, EmulatorService} from "../bindings/github.com/x1nx3r/cache-22-client/gui";
import {
    FilledButton, OutlinedButton, TextButton, FilledIconButton, IconButton, ElevatedCard,
    Dialog, LinearProgress, FilledSelect, SelectOption,
    OutlinedTextField,
} from './mwc.jsx';

export function fmtBytes(n) {
    if (n < 1024) return n + " B";
    const units = ["KB", "MB", "GB", "TB"];
    let i = -1;
    do { n /= 1024; i++; } while (n >= 1024 && i < units.length - 1);
    return n.toFixed(1) + " " + units[i];
}

function BindPill({binding}) {
    if (!binding) return <span className="bind-pill unmapped">unmapped</span>;
    const joy = binding.startsWith("SDL-");
    const short = binding.split("/")[1] || binding;
    return (
        <span className={"bind-pill" + (joy ? " joy" : " kbd")}>
            <span className="material-symbols-outlined">{joy ? "gamepad" : "keyboard"}</span>
            <span>{short}</span>
        </span>
    );
}

function GameCard({game, status, cover, onPlay, onFetch, onFull}) {
    const done = status ? status.done : game.done;
    const total = status ? status.total : game.total;
    const running = status && status.running;
    return (
        <ElevatedCard className="game-card">
            {cover
                ? <img src={cover} loading="lazy" alt=""/>
                : <div className="game-tile">{(game.title || "?").slice(0, 1).toUpperCase()}</div>}
            <div className="game-meta">
                <div className="game-title">{game.title}</div>
                <div className="game-sub">{game.serial} · {fmtBytes(game.size)}</div>
                <LinearProgress value={total > 0 ? done / total : 0}/>
                <div className="game-sub">{fmtBytes(done)} / {fmtBytes(total)}{running ? " · downloading…" : ""}</div>
                <div className="game-actions">
                    <FilledButton onClick={() => onPlay(game)}>Play</FilledButton>
                    <OutlinedButton onClick={() => onFetch(game)}>Fetch</OutlinedButton>
                    <OutlinedButton onClick={() => onFull(game)}>Full</OutlinedButton>
                </div>
            </div>
        </ElevatedCard>
    );
}

function qtName(code) {
    if (code.startsWith("Key")) return code.slice(3);
    if (code.startsWith("Digit")) return code.slice(5);
    if (code.startsWith("F") && /^F\d{1,2}$/.test(code)) return code;
    const map = {
        ArrowUp: "Up", ArrowDown: "Down", ArrowLeft: "Left", ArrowRight: "Right",
        Space: "Space", Enter: "Return", NumpadEnter: "Enter",
        Tab: "Tab", Backspace: "Backspace",
        ShiftLeft: "Shift", ShiftRight: "Shift",
        ControlLeft: "Control", ControlRight: "Control",
        AltLeft: "Alt", AltRight: "Alt",
        MetaLeft: "Meta", MetaRight: "Meta",
        CapsLock: "CapsLock",
        Minus: "Minus", Equal: "Equal",
        BracketLeft: "BracketLeft", BracketRight: "BracketRight",
        Backslash: "Backslash", Semicolon: "Semicolon",
        Quote: "Apostrophe", Backquote: "QuoteLeft",
        Comma: "Comma", Period: "Period", Slash: "Slash",
        Insert: "Insert", Delete: "Delete",
        Home: "Home", End: "End", PageUp: "PageUp", PageDown: "PageDown",
    };
    return map[code] || null;
}

function ServersView({servers, activeUrl, onUse, onRemove, onAdd, onTest, msg}) {
    const [name, setName] = useState("");
    const [url, setUrl] = useState("");
    const [user, setUser] = useState("");
    const [pass, setPass] = useState("");
    return (
        <div>
            <h1 className="headline">Servers</h1>
            <ElevatedCard className="form-card">
                <div className="card-pad">
                    <OutlinedTextField label="Name" placeholder="home-nas" value={name} onInput={(e) => setName(e.target.value)}/>
                    <OutlinedTextField label="URL" placeholder="http://192.168.1.10:8080" value={url} onInput={(e) => setUrl(e.target.value)}/>
                    <OutlinedTextField label="Username" value={user} onInput={(e) => setUser(e.target.value)}/>
                    <OutlinedTextField label="Password" type="password" value={pass} onInput={(e) => setPass(e.target.value)}/>
                    <div className="row">
                        <OutlinedButton onClick={() => onTest(url, user, pass)}>Test</OutlinedButton>
                        <FilledButton onClick={() => {
                            onAdd(name, url, user, pass);
                            setName(""); setUrl(""); setUser(""); setPass("");
                        }}>Add</FilledButton>
                    </div>
                    <p className="msg">{msg}</p>
                </div>
            </ElevatedCard>
            <div className="srv-list">
                {servers.map((s) => (
                    <div className="srv-row" key={s.url}>
                        <div className="grow">
                            <div className="srv-name">{s.name}{s.url === activeUrl ? " ●" : ""}</div>
                            <div className="srv-url">{s.url}{s.username ? " · " + s.username : ""}</div>
                        </div>
                        <OutlinedButton onClick={() => onUse(s.url)}>Use</OutlinedButton>
                        <OutlinedButton onClick={() => onRemove(s.url)}>Remove</OutlinedButton>
                    </div>
                ))}
            </div>
        </div>
    );
}

function SettingsCard({icon, title, sub, onClick}) {
    return (
        <ElevatedCard className="settings-card" onClick={onClick}>
            <div className="settings-card-inner">
                <span className="material-symbols-outlined settings-card-icon">{icon}</span>
                <div className="settings-card-text">
                    <div className="settings-card-title">{title}</div>
                    <div className="settings-card-sub">{sub}</div>
                </div>
                <span className="material-symbols-outlined settings-card-arrow">chevron_right</span>
            </div>
        </ElevatedCard>
    );
}

const PAD_GROUPS = [
    {title: "Face", buttons: ["Triangle", "Circle", "Cross", "Square"]},
    {title: "D-Pad", buttons: ["Up", "Down", "Left", "Right"]},
    {title: "Left stick", buttons: ["LUp", "LDown", "LLeft", "LRight", "L3"]},
    {title: "Right stick", buttons: ["RUp", "RDown", "RLeft", "RRight", "R3"]},
    {title: "Shoulders", buttons: ["L1", "L2", "R1", "R2"]},
    {title: "System", buttons: ["Select", "Start"]},
];

const GLYPHS = {
    Triangle: "\u25B3", Circle: "\u25CB", Cross: "\u2715", Square: "\u25A1",
    Up: "\u2191", Down: "\u2193", Left: "\u2190", Right: "\u2192",
    LUp: "\u2191", LDown: "\u2193", LLeft: "\u2190", LRight: "\u2192",
    RUp: "\u2191", RDown: "\u2193", RLeft: "\u2190", RRight: "\u2192",
};

// Stock PCSX2 keyboard layout, used by Reset defaults.
const DEFAULT_PAD_MAP = {
    Up: "Keyboard/Up", Right: "Keyboard/Right", Down: "Keyboard/Down", Left: "Keyboard/Left",
    Triangle: "Keyboard/I", Circle: "Keyboard/L", Cross: "Keyboard/K", Square: "Keyboard/J",
    Select: "Keyboard/Backspace", Start: "Keyboard/Return",
    L1: "Keyboard/Q", L2: "Keyboard/1", R1: "Keyboard/E", R2: "Keyboard/3",
    L3: "Keyboard/2", R3: "Keyboard/4",
    LUp: "Keyboard/W", LRight: "Keyboard/D", LDown: "Keyboard/S", LLeft: "Keyboard/A",
    RUp: "Keyboard/T", RRight: "Keyboard/H", RDown: "Keyboard/G", RLeft: "Keyboard/F",
};

function SettingsView({tab, setTab, games, saves, refreshSaves, dataDir, emuVersion, biosDir, biosOk, tier, link, padButtons, padMap, emuRunning, servers, setView, padSource, onPadSource, joyDevices, joyIndex, onJoyIndex, onProbe, onClean, onCleanAll, onImportBIOS, onBind, onCaptureJoy, onSyncSaves, onRestoreSave, onDeleteSave, notify}) {
    const [capturing, setCapturing] = useState(null);
    const [joyBusy, setJoyBusy] = useState(null);
    const boundCount = padButtons.filter((b) => padMap && padMap[b]).length;
    const resetPadDefaults = async () => {
        if (emuRunning) { notify("Stop the emulator to remap."); return; }
        if (!window.confirm("Reset all 24 bindings to PCSX2 defaults?")) return;
        try {
            for (const b of Object.keys(DEFAULT_PAD_MAP)) {
                await onBind(b, DEFAULT_PAD_MAP[b]);
            }
        } catch (e) { notify(e); }
    };
    useEffect(() => {
        if (!capturing) return;
        const h = async (e) => {
            e.preventDefault();
            e.stopPropagation();
            if (e.code === "Escape") { setCapturing(null); return; }
            const q = qtName(e.code);
            if (!q) { notify("That key can't be mapped."); return; }
            await onBind(capturing, "Keyboard/" + q);
            setCapturing(null);
        };
        window.addEventListener("keydown", h, true);
        return () => window.removeEventListener("keydown", h, true);
    }, [capturing]);
    const startCapture = async (b) => {
        if (emuRunning || capturing === b || joyBusy) return;
        if (padSource === "keyboard") { setCapturing(b); return; }
        setJoyBusy(b);
        await onCaptureJoy(b);
        setJoyBusy(null);
    };
    const totalDone = games.reduce((a, g) => a + (g.done || 0), 0);
    const totalSize = games.reduce((a, g) => a + (g.total || g.size || 0), 0);

    if (tab) {
        return (
            <div>
                <div className="settings-back" onClick={() => setTab(null)}>
                    <span className="material-symbols-outlined">arrow_back</span>
                    <span>Settings</span>
                </div>
                {tab === "storage" && (
                    <div>
                        <h1 className="headline">Storage</h1>
                        <ElevatedCard className="form-card">
                            <div className="card-pad">
                                <div className="kv"><span className="k">Data folder</span><span className="mono">{dataDir || "—"}</span></div>
                                <div className="kv"><span className="k">Cached</span><span>{fmtBytes(totalDone)} / {fmtBytes(totalSize)}</span></div>
                                <LinearProgress value={totalSize > 0 ? totalDone / totalSize : 0}/>
                                {games.map((g) => (
                                    <div className="srv-row" key={g.serial}>
                                        <div className="grow">
                                            <div className="srv-name">{g.title}</div>
                                            <div className="srv-url">{fmtBytes(g.done || 0)} / {fmtBytes(g.total || g.size || 0)}</div>
                                        </div>
                                        <OutlinedButton onClick={() => onClean(g.serial, g.title)}>Clean</OutlinedButton>
                                    </div>
                                ))}
                                <div className="row">
                                    <OutlinedButton onClick={onCleanAll}>Clean all</OutlinedButton>
                                </div>
                            </div>
                        </ElevatedCard>
                    </div>
                )}
                {tab === "emulator" && (
                    <div>
                        <h1 className="headline">Emulator</h1>
                        <ElevatedCard className="form-card">
                            <div className="card-pad">
                                <div className="kv"><span className="k">PCSX2</span><span className="mono">{emuVersion || "—"}</span></div>
                                <div className="kv"><span className="k">BIOS</span><span>{biosOk === null ? "?" : biosOk ? "installed" : "missing"}</span></div>
                                <div className="kv"><span className="k">BIOS folder</span><span className="mono">{biosDir || "—"}</span></div>
                                <div className="row">
                                    <OutlinedButton onClick={onImportBIOS}>Import BIOS…</OutlinedButton>
                                </div>
                            </div>
                        </ElevatedCard>
                    </div>
                )}
                {tab === "network" && (
                    <div>
                        <h1 className="headline">Network</h1>
                        <ElevatedCard className="form-card">
                            <div className="card-pad">
                                <div className="kv"><span className="k">Last probe</span><span>{link || "never — press play or probe now"}</span></div>
                                <div className="kv"><span className="k">Fetch tier</span><span>{tier ? fmtBytes(tier.fetchSize) + " × " + tier.prefetchDepth + (tier.probed ? " · " + tier.class : " · defaults") : "—"}</span></div>
                                <div className="row">
                                    <OutlinedButton onClick={onProbe}>Probe now</OutlinedButton>
                                </div>
                            </div>
                        </ElevatedCard>
                    </div>
                )}
                {tab === "saves" && (
                    <div>
                        <h1 className="headline">Saves</h1>
                        <p className="msg">Per-game memory cards, both slots. Sync pulls newer cloud copies; restore brings back a local backup; delete removes local and cloud copies.</p>
                        <div className="saves-grid">
                            {saves.map((g) => (
                                <div className="save-game" key={g.serial}>
                                    <div className="save-game-head">
                                        <div className="grow">
                                            <div className="srv-name">{titleFor(g.serial)}</div>
                                            <div className="srv-url">{g.serial}</div>
                                        </div>
                                        <OutlinedButton onClick={() => onSyncSaves(g.serial)}>Sync</OutlinedButton>
                                    </div>
                                    {g.slots.map((s) => (
                                        <div key={s.slot}>
                                            <div className="pad-row" style={{cursor: "default"}}>
                                                <span className="pad-glyph-badge">S{s.slot}</span>
                                                <span className="pad-row-name">
                                                    {s.present ? fmtBytes(s.size) + " · " + fmtDate(s.mtime) : "empty"}
                                                </span>
                                                <span className="grow"/>
                                                {!s.present
                                                    ? <span className="save-pill empty">Empty</span>
                                                    : s.synced
                                                        ? <span className="save-pill synced">Synced</span>
                                                        : <span className="save-pill local">Local changes</span>}
                                                {s.present && (
                                                    <span className="pad-clear" title="Delete save" onClick={() => onDeleteSave(g.serial, s.slot)}>
                                                        <span className="material-symbols-outlined">delete</span>
                                                    </span>
                                                )}
                                            </div>
                                            {s.backups.map((b) => (
                                                <div className="bak-row" key={b.name}>
                                                    <span className="material-symbols-outlined">history</span>
                                                    <span>{fmtDate(b.mtime)} · {fmtBytes(b.size)}</span>
                                                    <span className="grow"/>
                                                    <span className="bak-restore" onClick={() => onRestoreSave(g.serial, s.slot, b.name)}>Restore</span>
                                                </div>
                                            ))}
                                        </div>
                                    ))}
                                </div>
                            ))}
                        </div>
                        {saves.length === 0 && <p className="empty">No saves yet — play a game first.</p>}
                    </div>
                )}
                {tab === "controller" && (
                    <div>
                        <h1 className="headline">Controller</h1>
                        <div className="pad-head">
                            <div className="row">
                                {padSource === "keyboard"
                                    ? <FilledButton>Keyboard</FilledButton>
                                    : <OutlinedButton onClick={() => onPadSource("keyboard")}>Keyboard</OutlinedButton>}
                                {padSource === "gamepad"
                                    ? <FilledButton>Gamepad</FilledButton>
                                    : <OutlinedButton onClick={() => onPadSource("gamepad")}>Gamepad</OutlinedButton>}
                            </div>
                            {padSource === "gamepad" && (
                                <FilledSelect label="Gamepad" value={String(joyIndex)} onChange={(e) => onJoyIndex(Number(e.target.value))}>
                                    {joyDevices.length === 0
                                        ? <SelectOption value="0"><div slot="headline">No gamepads found</div></SelectOption>
                                        : joyDevices.map((d) => (
                                            <SelectOption key={d.index} value={String(d.index)}>
                                                <div slot="headline">{d.name}</div>
                                            </SelectOption>
                                        ))}
                                </FilledSelect>
                            )}
                            <span className="grow"/>
                            <span className="msg">{boundCount}/24 bound</span>
                            <OutlinedButton onClick={resetPadDefaults}>Reset defaults</OutlinedButton>
                        </div>
                        {emuRunning && <p className="msg">Stop the emulator to remap.</p>}
                        <div className="pad-groups">
                            {PAD_GROUPS.map((g) => (
                                <div className="pad-group" key={g.title}>
                                    <div className="pad-group-title">{g.title}</div>
                                    {g.buttons.map((b) => (
                                        <div
                                            className={"pad-row" + (capturing === b || joyBusy === b ? " capturing" : "")}
                                            key={b}
                                            onClick={() => startCapture(b)}
                                            style={emuRunning ? undefined : {cursor: "pointer"}}
                                        >
                                            {GLYPHS[b] && <span className="pad-glyph-badge">{GLYPHS[b]}</span>}
                                            <span className="pad-row-name">{b}</span>
                                            <span className="grow"/>
                                            {capturing === b
                                                ? <span className="msg">key…</span>
                                                : joyBusy === b
                                                    ? <span className="msg">pad…</span>
                                                    : <BindPill binding={padMap && padMap[b]}/>}
                                            {padMap && padMap[b] && capturing !== b && !joyBusy && !emuRunning && (
                                                <span className="pad-clear" onClick={(e) => { e.stopPropagation(); onBind(b, ""); }} title="Clear">
                                                    <span className="material-symbols-outlined">close</span>
                                                </span>
                                            )}
                                        </div>
                                    ))}
                                </div>
                            ))}
                        </div>
                    </div>
                )}
            </div>
        );
    }

    return (
        <div>
            <h1 className="headline">Settings</h1>
            <div className="settings-grid">
                <SettingsCard icon="storage" title="Storage" sub={fmtBytes(totalDone) + " / " + fmtBytes(totalSize) + " cached"} onClick={() => setTab("storage")}/>
                <SettingsCard icon="precision_manufacturing" title="Emulator" sub={emuVersion || "not installed"} onClick={() => setTab("emulator")}/>
                <SettingsCard icon="dns" title="Servers" sub={servers.length + " server" + (servers.length !== 1 ? "s" : "")} onClick={() => setView("servers")}/>
                <SettingsCard icon="memory" title="BIOS" sub={biosOk === null ? "unknown" : biosOk ? "installed" : "missing"} onClick={() => setTab("emulator")}/>
                <SettingsCard icon="network_check" title="Network" sub={link || "no probe yet"} onClick={() => setTab("network")}/>
                <SettingsCard icon="gamepad" title="Controller" sub={padButtons.length + " bindings"} onClick={() => setTab("controller")}/>
                <SettingsCard icon="save" title="Saves" sub={saves.length + " game" + (saves.length !== 1 ? "s" : "")} onClick={() => { refreshSaves(); setTab("saves"); }}/>
            </div>
        </div>
    );
}
export default function App() {
    const [view, setView] = useState("library");
    const [settingsTab, setSettingsTab] = useState(null);
    const [servers, setServers] = useState([]);
    const [activeUrl, setActiveUrl] = useState("");
    const [games, setGames] = useState([]);
    const [emptyMsg, setEmptyMsg] = useState("");
    const [statuses, setStatuses] = useState({});
    const [biosOk, setBiosOk] = useState(null);
    const [link, setLink] = useState("");
    const [player, setPlayer] = useState(null);
    const [playerStatus, setPlayerStatus] = useState(null);
    const [covers, setCovers] = useState({});
    const [srvMsg, setSrvMsg] = useState("");
    const [emu, setEmu] = useState({running: false});
    const [tier, setTier] = useState(null);
    const [dataDir, setDataDir] = useState("");
    const [emuVersion, setEmuVersion] = useState("");
    const [biosDir, setBiosDir] = useState("");
    const [padButtons, setPadButtons] = useState([]);
    const [padMap, setPadMap] = useState({});
    const [padSource, setPadSource] = useState("keyboard");
    const [joyDevices, setJoyDevices] = useState([]);
    const [joyIndex, setJoyIndex] = useState(0);
    const [saves, setSaves] = useState([]);
    const [toasts, setToasts] = useState([]);
    const dialogRef = useRef(null);
    const serversRef = useRef([]);
    serversRef.current = servers;
    const lastSaveSeq = useRef(0);
    const lastDlErrs = useRef({});

    const toast = useCallback((msg, kind) => {
        kind = kind || "info";
        const id = Date.now() + Math.random();
        setToasts((prev) => [...prev.slice(-2), {id, kind, text: String(msg).slice(0, 220)}]);
        setTimeout(() => {
            setToasts((prev) => prev.filter((t) => t.id !== id));
        }, kind === "error" ? 9000 : 5000);
    }, []);

    const dismissToast = useCallback((id) => {
        setToasts((prev) => prev.filter((t) => t.id !== id));
    }, []);

    const activeServer = useCallback(() => {
        return serversRef.current.find((s) => s.url === activeUrl)
            || serversRef.current.find((s) => s.url) || null;
    }, [activeUrl]);

    const coverURL = useCallback(async (serial) => {
        const srv = serversRef.current.find((s) => s.url === activeUrl) || serversRef.current[0];
        if (!srv) return null;
        const res = await fetch(srv.url + "/v1/games/" + encodeURIComponent(serial) + "/cover", {
            headers: {Authorization: "Bearer " + srv.token},
        });
        if (!res.ok) return null;
        return URL.createObjectURL(await res.blob());
    }, [activeUrl]);

    const refreshServers = useCallback(async () => {
        try {
            const list = await ServerService.Servers();
            setServers(list);
            setActiveUrl(await ServerService.Active());
        } catch (e) { console.error(e); }
    }, []);

    const loadLibrary = useCallback(async () => {
        let list;
        try {
            list = await ServerService.Servers();
        } catch (e) { /* fall through */ }
        const known = list || serversRef.current;
        if (known.length === 0) {
            setGames([]);
            setEmptyMsg("No servers yet — open Servers and add yours.");
            return;
        }
        const act = known.find((s) => s.url === activeUrl) || known[0];
        if (act && !act.token) {
            setGames([]);
            setEmptyMsg("This server was added before logins existed — remove it and add it again with your username.");
            return;
        }
        try {
            const gs = await LibraryService.Games();
            setGames(gs);
            setEmptyMsg(gs.length === 0 ? "No games on this server." : "");
            const arts = {};
            await Promise.all(gs.map(async (g) => {
                const srv = (list || serversRef.current).find((s) => s.url === activeUrl) || (list || serversRef.current)[0];
                if (!srv) return;
                try {
                    const res = await fetch(srv.url + "/v1/games/" + encodeURIComponent(g.serial) + "/cover", {
                        headers: {Authorization: "Bearer " + srv.token},
                    });
                    if (res.ok) arts[g.serial] = URL.createObjectURL(await res.blob());
                } catch (e) { /* no art */ }
            }));
            setCovers(arts);
        } catch (e) {
            const msg = String(e);
            setGames([]);
            setEmptyMsg(msg.includes("401")
                ? "Login expired — remove this server and add it again."
                : msg);
        }
    }, [activeUrl]);

    const refreshPad = useCallback(async () => {
        try {
            setPadButtons(await EmulatorService.PadButtons());
            setPadMap(await EmulatorService.PadBindings() || {});
        } catch (e) { /* ignore */ }
    }, []);

    const bindPad = async (button, binding) => {
        try {
            await EmulatorService.SetPadBinding(button, binding);
            refreshPad();
        } catch (e) { toast(e); }
    };

    const refreshJoy = useCallback(async () => {
        try {
            const devs = await EmulatorService.JoyDevices();
            setJoyDevices(devs || []);
            if (devs && devs.length > 0) setJoyIndex(devs[0].index);
        } catch (e) { /* ignore */ }
    }, []);

    const captureJoy = async (button) => {
        try {
            const token = await EmulatorService.CaptureJoy(joyIndex);
            await EmulatorService.SetPadBinding(button, token);
            refreshPad();
        } catch (e) { toast(e); }
    };

    const refreshSaves = useCallback(async () => {
        try {
            setSaves(await EmulatorService.ListSaves() || []);
        } catch (e) { /* ignore */ }
    }, []);

    const titleFor = useCallback((serial) => {
        const g = games.find((x) => x.serial === serial);
        return g ? g.title : serial;
    }, [games]);

    const syncSave = async (serial) => {
        try {
            await EmulatorService.SyncSaves(serial);
            refreshSaves();
        } catch (e) { toast(e, "error"); }
    };

    const restoreSave = async (serial, slot, name) => {
        if (!window.confirm("Restore this backup over the current card?")) return;
        try {
            await EmulatorService.RestoreSave(serial, slot, name);
            refreshSaves();
        } catch (e) { toast(e, "error"); }
    };

    const deleteSave = async (serial, slot) => {
        if (!window.confirm("Delete this save locally and in the cloud?")) return;
        try {
            await EmulatorService.DeleteSave(serial, slot);
            refreshSaves();
        } catch (e) { toast(e, "error"); }
    };

    const biosStatus = useCallback(async () => {
        try {
            setBiosOk(await EmulatorService.BiosOK());
        } catch (e) { setBiosOk(null); }
    }, []);

    const refreshLink = useCallback(async () => {
        try {
            const p = await EmulatorService.LastProbe();
            if (!p) return;
            setLink(p.class + " · " + Math.round(p.rttMs) + "ms · " + Math.round(p.mbps) + "Mbps");
        } catch (e) { /* no probe yet */ }
    }, []);

    useEffect(() => {
        refreshServers();
        biosStatus();
        refreshLink();
        (async () => {
            try { setDataDir(await LibraryService.DataDir()); } catch (e) { /* ignore */ }
            try { setEmuVersion(await EmulatorService.Version()); } catch (e) { /* ignore */ }
            try { setBiosDir(await EmulatorService.BiosDir()); } catch (e) { /* ignore */ }
            try { setTier(await EmulatorService.FetchTier()); } catch (e) { /* ignore */ }
            try { setEmu(await EmulatorService.EmuStatus()); } catch (e) { /* ignore */ }
            refreshPad();
            refreshJoy();
            refreshSaves();
        })();
    }, [refreshServers, biosStatus, refreshLink]);

    useEffect(() => { loadLibrary(); }, [loadLibrary]);

    useEffect(() => {
        if (emu.saveSeq && emu.saveSeq !== lastSaveSeq.current) {
            lastSaveSeq.current = emu.saveSeq;
            toast(emu.saveMsg, emu.saveOk ? "success" : "error");
        }
    }, [emu]);

    useEffect(() => {
        const tick = async () => {
            try {
                const updates = {};
                for (const g of games) {
                    try {
                        updates[g.serial] = await DownloadService.Status(g.serial);
                    } catch (e) { /* ignore */ }
                }
                if (Object.keys(updates).length) {
                    setStatuses((prev) => ({...prev, ...updates}));
                }
                for (const [serial, s] of Object.entries(updates)) {
                    if (s.lastError && lastDlErrs.current[serial] !== s.lastError) {
                        lastDlErrs.current[serial] = s.lastError;
                        toast(s.lastError, "error");
                    }
                }
                if (player) {
                    try {
                        setPlayerStatus(await DownloadService.Status(player.serial));
                        refreshLink();
                    } catch (e) { /* ignore */ }
                }
                try {
                    setEmu(await EmulatorService.EmuStatus());
                } catch (e) { /* ignore */ }
            } catch (e) { /* ignore */ }
        };
        const timer = setInterval(tick, 2000);
        return () => clearInterval(timer);
    }, [games, player, refreshLink]);

    const doPlay = async (game) => {
        if (emu && emu.running) {
            toast("Already playing " + (emu.title || emu.serial) + " — stop it first.", "error");
            return;
        }
        setPlayer({serial: game.serial, title: game.title});
        try {
            await EmulatorService.Play(game.serial);
            refreshLink();
        } catch (e) {
            if (String(e).includes("BIOS")) {
                dialogRef.current && dialogRef.current.show();
            } else {
                toast(e, "error");
            }
        }
    };

    const stopEmu = async () => {
        try {
            await EmulatorService.StopEmulator();
            setEmu(await EmulatorService.EmuStatus());
        } catch (e) { toast(e, "error"); }
    };

    const probeNow = async () => {
        try {
            await EmulatorService.ProbeNow();
            refreshLink();
            setTier(await EmulatorService.FetchTier());
            setSrvMsg("Probe complete.");
        } catch (e) { setSrvMsg("Probe failed: " + e); }
    };

    const cleanGame = async (serial, title) => {
        if (!window.confirm("Delete cached data for " + title + "?")) return;
        try {
            await LibraryService.Clean(serial);
            loadLibrary();
        } catch (e) { toast(e, "error"); }
    };

    const cleanAll = async () => {
        if (!window.confirm("Delete ALL cached game data?")) return;
        try {
            await LibraryService.Clean("all");
            loadLibrary();
        } catch (e) { toast(e, "error"); }
    };

    const fmtTime = (unix) => {
        try {
            return new Date(unix * 1000).toLocaleTimeString([], {hour: "2-digit", minute: "2-digit", second: "2-digit"});
        } catch (e) { return ""; }
    };

    const fmtDate = (unix) => {
        try {
            return new Date(unix * 1000).toLocaleDateString([], {month: "short", day: "numeric"}) + " " +
                new Date(unix * 1000).toLocaleTimeString([], {hour: "2-digit", minute: "2-digit"});
        } catch (e) { return ""; }
    };

    const importBIOS = async () => {
        try {
            const n = await EmulatorService.ImportBIOS();
            if (n > 0) {
                dialogRef.current && dialogRef.current.close();
                toast("Installed " + n + " BIOS file(s).", "success");
                biosStatus();
            }
        } catch (e) { toast(e, "error"); }
    };

    return (
        <div className="app">
            <aside className="rail">
                <div className="brand">
                    <img src="/wails.png" alt="" className="brand-logo"/>
                </div>
                <nav className="nav">
                    {[
                        {id: "library", icon: "sports_esports", label: "Library", go: () => { setView("library"); loadLibrary(); }},
                        {id: "servers", icon: "dns", label: "Servers", go: () => setView("servers")},
                        {id: "settings", icon: "settings", label: "Settings", go: () => { setView("settings"); setSettingsTab(null); }},
                    ].map((item) => (
                        <IconButton
                            key={item.id}
                            className={"nav-btn" + (view === item.id ? " nav-active" : "")}
                            aria-label={item.label}
                            title={item.label}
                            onClick={item.go}
                        >
                            <span className="material-symbols-outlined">{item.icon}</span>
                        </IconButton>
                    ))}
                </nav>
            </aside>

            <main>
                {view === "library" && (
                    <div>
                        <h1 className="headline">Your library</h1>
                        <div className="grid">
                            {games.map((g) => (
                                <GameCard
                                    key={g.serial}
                                    game={g}
                                    status={statuses[g.serial]}
                                    cover={covers[g.serial]}
                                    onPlay={doPlay}
                                    onFetch={async (game) => {
                                        try { await DownloadService.FetchBoot(game.serial); }
                                        catch (e) { toast(e); }
                                    }}
                                    onFull={async (game) => {
                                        try { await DownloadService.SyncFull(game.serial); }
                                        catch (e) { toast(e); }
                                    }}
                                />
                            ))}
                        </div>
                        {emptyMsg && <p className="empty">{emptyMsg}</p>}
                    </div>
                )}
                {view === "servers" && (
                    <ServersView
                        servers={servers}
                        activeUrl={activeUrl}
                        msg={srvMsg}
                        onUse={async (url) => {
                            await ServerService.SetActive(url);
                            setActiveUrl(url);
                            loadLibrary();
                        }}
                        onRemove={async (url) => {
                            await ServerService.RemoveServer(url);
                            refreshServers();
                            loadLibrary();
                        }}
                        onTest={async (url, user, pass) => {
                            try {
                                await ServerService.TestConnection(url, user, pass);
                                setSrvMsg(user ? "OK — username and password accepted." : "OK — server reachable.");
                            } catch (e) { setSrvMsg("Failed: " + e); }
                        }}
                        onAdd={async (name, url, user, pass) => {
                            try {
                                await ServerService.AddServer(name, url, user, pass);
                                refreshServers();
                                loadLibrary();
                            } catch (e) { toast(e); }
                        }}
                    />
                )}
                {view === "settings" && (
                    <SettingsView
                        tab={settingsTab}
                        setTab={setSettingsTab}
                        games={games}
                        dataDir={dataDir}
                        emuVersion={emuVersion}
                        biosDir={biosDir}
                        biosOk={biosOk}
                        tier={tier}
                        link={link}
                        saves={saves}
                        refreshSaves={refreshSaves}
                        padButtons={padButtons}
                        padMap={padMap}
                        emuRunning={emu.running}
                        servers={servers}
                        setView={setView}
                        padSource={padSource}
                        onPadSource={setPadSource}
                        joyDevices={joyDevices}
                        joyIndex={joyIndex}
                        onJoyIndex={setJoyIndex}
                        onProbe={probeNow}
                        onClean={cleanGame}
                        onCleanAll={cleanAll}
                        onImportBIOS={importBIOS}
                        onBind={bindPad}
                        onCaptureJoy={captureJoy}
                        onSyncSaves={syncSave}
                        onRestoreSave={restoreSave}
                        onDeleteSave={deleteSave}
                        notify={toast}
                    />
                )}
            </main>

            <footer className="player">
                {emu.running ? (
                    <img className="player-art" alt="" src={covers[emu.serial] ? covers[emu.serial] : ""}/>
                ) : playerStatus || player || emu.stage ? (
                    <img className="player-art" alt="" src={player && covers[player.serial] ? covers[player.serial] : ""}/>
                ) : <div className="player-art"/>}
                <div className="player-meta">
                    <div className="player-title">
                        {emu.running
                            ? (emu.title || emu.serial || "Playing")
                            : emu.stage && !player
                                ? "Preparing…"
                                : player ? player.title : "Nothing playing"}
                    </div>
                    {emu.running ? (
                        <div className="player-sub">running since {fmtTime(emu.sinceUnix)}</div>
                    ) : emu.stage ? (
                        <LinearProgress value={emu.stageTotal > 0 ? emu.stageDone / emu.stageTotal : 0}/>
                    ) : (
                        <LinearProgress value={playerStatus && playerStatus.total > 0 ? playerStatus.done / playerStatus.total : 0}/>
                    )}
                    {!emu.running && (
                        <div className={"player-sub" + (emu.stage && emu.stage.startsWith("Failed") ? " failed" : "")}>
                            {emu.stage
                                ? emu.stage + (emu.stageTotal > 0
                                    ? " — " + fmtBytes(emu.stageDone) + " / " + fmtBytes(emu.stageTotal)
                                    : "")
                                : playerStatus
                                    ? fmtBytes(playerStatus.done) + " / " + fmtBytes(playerStatus.total) + (playerStatus.running ? " · downloading…" : "")
                                    : ""}
                        </div>
                    )}
                </div>
                {emu.running ? (
                    <FilledIconButton aria-label="Stop" onClick={stopEmu}>
                        <span className="material-symbols-outlined">stop</span>
                    </FilledIconButton>
                ) : (
                    <FilledIconButton aria-label="Play" onClick={() => { if (player) doPlay(player); }}>
                        <span className="material-symbols-outlined">play_arrow</span>
                    </FilledIconButton>
                )}
            </footer>

            <Dialog ref={dialogRef}>
                <span slot="headline">No PS2 BIOS found</span>
                <span slot="content">
                    Cache-22 never ships a BIOS. Dump yours from a real PlayStation 2,
                    then install it with one click. Files land in your Cache-22 data
                    folder, where its private PCSX2 setup expects them.
                </span>
                <span slot="actions">
                    <TextButton formMethod="dialog" value="later">Later</TextButton>
                    <FilledButton onClick={importBIOS} autoFocus>Select BIOS file…</FilledButton>
                </span>
            </Dialog>

            <div className="toasts">
                {toasts.map((t) => (
                    <div key={t.id} className={"toast " + t.kind} onClick={() => dismissToast(t.id)}>
                        <span className="material-symbols-outlined">
                            {t.kind === "error" ? "error" : t.kind === "success" ? "check_circle" : "info"}
                        </span>
                        <span>{t.text}</span>
                    </div>
                ))}
            </div>
        </div>
    );
}
