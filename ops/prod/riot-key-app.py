#!/usr/bin/env python3
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
from queue import Empty, SimpleQueue
import tkinter as tk
from tkinter import font, messagebox, ttk

from riot_key_client import Target, UpdaterError, load_target, save_target, update_key, validate_key, verify_target


BACKGROUND = "#111416"
PANEL = "#1b2024"
BORDER = "#35444d"
TEXT = "#f4f6f8"
MUTED = "#a9c5ce"
ACCENT = "#5385ff"
ERROR = "#ffa6a6"
SUCCESS = "#9ddcb9"
LOG_LINES = 2000
APP_DIR = Path(__file__).resolve().parent
ICON = APP_DIR / "winrift-icon.png"
if not ICON.is_file():
    ICON = APP_DIR.parents[1] / "apps/web/public/images/brand/winrift-icon.png"


class KeyUpdater:
    def __init__(self, root):
        self.root = root
        self.executor = ThreadPoolExecutor(max_workers=1)
        self.output = SimpleQueue()
        self.pending = None
        self.address = tk.StringVar(master=root)
        self.username = tk.StringVar(master=root)
        self.hostname = tk.StringVar(master=root)
        self.key = tk.StringVar(master=root)
        self.rollover = tk.BooleanVar(master=root)
        self.status = tk.StringVar(master=root, value="Enter your WinRift server details to get started.")

        root.title("WinRift · Riot key updater")
        root.configure(background=BACKGROUND)
        root.protocol("WM_DELETE_WINDOW", self.close)
        self.setup_styles()
        frame = ttk.Frame(root, padding=24)
        frame.pack(fill="both", expand=True)
        header = ttk.Frame(frame)
        header.pack(fill="x", pady=(0, 20))
        if ICON.is_file():
            self.icon = tk.PhotoImage(master=root, file=str(ICON))
            root.iconphoto(True, self.icon)
            scale = max(1, (max(self.icon.width(), self.icon.height()) + 43) // 44)
            self.logo = self.icon.subsample(scale, scale)
            ttk.Label(header, image=self.logo).pack(side="left", padx=(0, 12))
        title = ttk.Frame(header)
        title.pack(side="left")
        ttk.Label(title, text="WinRift", style="Title.TLabel").pack(anchor="w")
        ttk.Label(title, text="Riot key updater", foreground=MUTED).pack(anchor="w")

        server = self.panel(frame, "WinRift server")
        server.columnconfigure(0, weight=1, uniform="servers")
        server.columnconfigure(1, weight=1, uniform="servers")
        self.address_entry = self.field(server, "Server IP address", self.address, row=1, column=0, padx=(0, 6))
        self.hostname_entry = self.field(server, "Expected machine name", self.hostname, row=1, column=1, padx=(6, 0))
        self.username_entry = self.field(server, "SSH username", self.username, row=2, column=0, padx=(0, 6))
        self.verify_button = ttk.Button(server, text="Verify and save server", command=self.verify)
        self.verify_button.grid(row=2, column=1, sticky="sew", padx=(6, 0), pady=(0, 10))

        key_panel = self.panel(frame, "Riot API key")
        key_panel.columnconfigure(0, weight=1)
        self.key_entry = self.field(key_panel, "Paste a fresh key", self.key, row=1, column=0, show="*")
        self.rollover_check = ttk.Checkbutton(
            key_panel, text="Advance to the latest patch and archive older data", variable=self.rollover,
        )
        self.rollover_check.grid(row=2, column=0, sticky="w", pady=(0, 12))
        self.update_button = ttk.Button(
            key_panel, text="Update Riot key", style="Accent.TButton", command=self.update,
        )
        self.update_button.grid(row=3, column=0, sticky="e")

        activity = self.panel(frame, "Update log")
        activity.pack_configure(fill="both", expand=True)
        activity.columnconfigure(0, weight=1)
        activity.rowconfigure(1, weight=1)
        self.log = tk.Text(
            activity, height=8, width=44, wrap="word", state="disabled", font="TkFixedFont",
            background=BACKGROUND, foreground=TEXT, selectbackground="#365caa", selectforeground=TEXT,
            highlightbackground=BORDER, highlightcolor=ACCENT, highlightthickness=1, borderwidth=0, padx=8, pady=8,
        )
        scrollbar = ttk.Scrollbar(activity, orient="vertical", command=self.log.yview)
        self.log.configure(yscrollcommand=scrollbar.set)
        self.log.grid(row=1, column=0, sticky="nsew")
        scrollbar.grid(row=1, column=1, sticky="ns", padx=(4, 0))
        self.write_log("Server output appears here during a key update.\n")
        self.status_label = ttk.Label(frame, textvariable=self.status, foreground=MUTED, wraplength=500)
        self.status_label.pack(fill="x", pady=(2, 0))
        self.status_label.bind("<Configure>", lambda event: self.status_label.configure(wraplength=event.width))

        self.load_settings()
        for field in (self.address, self.username, self.hostname):
            field.trace_add("write", self.target_changed)
        self.key.trace_add("write", lambda *_: self.sync_controls())
        self.sync_controls()
        self.address_entry.focus_set()
        root.update_idletasks()
        root.minsize(max(552, root.winfo_reqwidth()), root.winfo_reqheight() + 18)

    def setup_styles(self):
        family = font.nametofont("TkDefaultFont", root=self.root).actual("family")
        settings = ttk.Style(self.root)
        settings.theme_use("clam")
        settings.configure(".", background=BACKGROUND, foreground=TEXT, font=(family, 10))
        settings.configure("Title.TLabel", font=(family, 22, "bold"))
        settings.configure("Panel.TFrame", background=PANEL, bordercolor=BORDER, relief="solid", borderwidth=1)
        settings.configure("Panel.TLabel", background=PANEL)
        settings.configure("Heading.TLabel", background=PANEL, font=(family, 12, "bold"))
        settings.configure("TEntry", fieldbackground=BACKGROUND, foreground=TEXT, insertcolor=TEXT,
                           bordercolor=BORDER, lightcolor=BACKGROUND, darkcolor=BACKGROUND, padding=7)
        settings.map("TEntry", bordercolor=[("focus", ACCENT)], fieldbackground=[("disabled", PANEL)])
        settings.configure("TButton", background=PANEL, bordercolor=BORDER, padding=(12, 8))
        settings.configure("Accent.TButton", background=ACCENT, foreground="#091323", bordercolor=ACCENT,
                           font=(family, 10, "bold"))
        settings.map("TButton", background=[("active", BORDER)], bordercolor=[("focus", ACCENT)])
        settings.map("Accent.TButton", background=[("disabled", BORDER), ("active", "#82acff")],
                     foreground=[("disabled", MUTED)])
        settings.configure("TCheckbutton", background=PANEL)
        settings.map("TCheckbutton", background=[("active", PANEL)],
                     indicatorcolor=[("selected", ACCENT), ("!selected", BACKGROUND)])

    def panel(self, parent, title):
        panel = ttk.Frame(parent, style="Panel.TFrame", padding=16)
        panel.pack(fill="x", pady=(0, 12))
        ttk.Label(panel, text=title, style="Heading.TLabel").grid(row=0, column=0, sticky="w", pady=(0, 12))
        return panel

    def field(self, panel, label, value, *, row, column, padx=0, show=""):
        group = ttk.Frame(panel, style="Panel.TFrame", relief="flat", borderwidth=0)
        group.grid(row=row, column=column, sticky="ew", padx=padx, pady=(0, 10))
        ttk.Label(group, text=label, style="Panel.TLabel", foreground=MUTED).pack(anchor="w", pady=(0, 4))
        entry = ttk.Entry(group, textvariable=value, show=show, width=22)
        entry.pack(fill="x")
        return entry

    def load_settings(self):
        try:
            target = load_target()
        except (OSError, ValueError, UpdaterError):
            self.set_status("Saved server details couldn’t be loaded. Enter them again.", ERROR)
            return
        if target:
            self.address.set(target.address)
            self.hostname.set(target.hostname)
            self.username.set(target.username)
            self.set_status(f"Paste a fresh key to update WinRift on {target.hostname}.")

    def set_status(self, text, color=MUTED):
        self.status.set(text)
        self.status_label.configure(foreground=color)

    def write_log(self, text, *, clear=False):
        follow = clear or self.log.yview()[1] >= 0.999
        self.log.configure(state="normal")
        if clear:
            self.log.delete("1.0", "end")
        self.log.insert("end", text)
        lines = int(self.log.index("end-1c").split(".")[0])
        if lines > LOG_LINES:
            self.log.delete("1.0", f"{lines - LOG_LINES + 1}.0")
        self.log.configure(state="disabled")
        if follow:
            self.log.see("end")

    def target_changed(self, *_):
        self.set_status("The app will verify these server details before sending a key.")

    def sync_controls(self):
        busy = self.pending is not None
        for widget in (self.address_entry, self.hostname_entry, self.username_entry, self.verify_button,
                       self.key_entry, self.rollover_check):
            widget.state(["disabled" if busy else "!disabled"])
        can_update = not busy and bool(self.key.get().strip())
        self.update_button.state(["!disabled" if can_update else "disabled"])

    def current_target(self):
        return Target.parse(self.address.get(), self.username.get(), self.hostname.get())

    def verify(self):
        try:
            target = self.current_target()
        except UpdaterError as error:
            self.set_status(str(error), ERROR)
            return

        def ready():
            try:
                save_target(target)
            except OSError:
                self.set_status("Server verified, but this computer couldn’t save its settings.", ERROR)
            else:
                self.set_status(f"Verified and saved {target.hostname} at {target.address}.", SUCCESS)
            self.key_entry.focus_set()

        self.start_task("Verifying server over SSH…", lambda: verify_target(target), ready)

    def update(self):
        try:
            target = self.current_target()
            key = validate_key(self.key.get())
        except UpdaterError as error:
            self.set_status(str(error), ERROR)
            return
        rollover = self.rollover.get()
        details = f"Update the Riot key on {target.hostname} ({target.address}) and restart the WinRift API and collector?"
        if rollover:
            details += "\n\nIf a new patch is available, WinRift will also advance to it and archive data outside the retention window."
        if not messagebox.askyesno("Confirm key update", details, default=messagebox.NO, parent=self.root):
            return
        self.key.set("")
        self.write_log(f"Checking SSH identity of {target.hostname} before sending the key.\n", clear=True)
        self.start_task(
            f"Updating WinRift on {target.hostname}. Leave this window open until it finishes.",
            lambda: update_key(target, key, rollover=rollover, on_output=self.output.put),
            lambda: self.set_status(f"Riot key updated on {target.hostname}. WinRift’s collector restarted.", SUCCESS),
        )

    def start_task(self, status, task, on_success):
        self.set_status(status)
        self.pending = self.executor.submit(task)
        self.sync_controls()
        self.root.after(80, self.finish_task, on_success)

    def finish_task(self, on_success):
        lines = []
        for _ in range(200):
            try:
                lines.append(self.output.get_nowait())
            except Empty:
                break
        if lines:
            self.write_log("".join(lines))
        if not self.pending.done() or not self.output.empty():
            self.root.after(80, self.finish_task, on_success)
            return
        try:
            self.pending.result()
        except UpdaterError as error:
            self.set_status(str(error), ERROR)
        except Exception:
            self.set_status("The operation ended unexpectedly. Check WinRift before trying again.", ERROR)
        else:
            on_success()
        finally:
            self.pending = None
            self.sync_controls()

    def close(self):
        if self.pending is not None:
            return
        self.executor.shutdown(wait=False)
        self.root.destroy()


if __name__ == "__main__":
    window = tk.Tk()
    KeyUpdater(window)
    window.mainloop()
