# rws shell wrapper
# Compatible with bash and zsh

rws() {
    if [ "$1" = "cd" ]; then
        shift
        local target
        if [ $# -eq 0 ]; then
            local state_dir="${XDG_STATE_HOME:-$HOME/.local/state}"
            local recent_file="$state_dir/rws/rws_recent_dir"
            if [ -f "$recent_file" ]; then
                target="$(cat "$recent_file")"
            else
                echo "rws cd: no recent workspace (run 'rws add' first)" >&2
                return 1
            fi
        elif [ "${1#/}" != "$1" ]; then
            # Absolute path
            target="$1"
        else
            # Relative name — resolve under workspace root
            local root
            root="$(command rws config get workspace_root 2>/dev/null)" || {
                echo "rws cd: failed to resolve workspace_root" >&2
                return 1
            }
            if [ -z "$root" ]; then
                echo "rws cd: workspace_root is empty" >&2
                return 1
            fi
            target="$root/$1"
        fi
        if [ -d "$target" ]; then
            cd "$target" || return 1
        else
            echo "rws cd: directory does not exist: $target" >&2
            return 1
        fi
    else
        RWS_SHELL_FUNCTION=1 command rws "$@"
    fi
}

# Bash completion
if [ -n "$BASH_VERSION" ]; then
    _rws_completions() {
        local cur="${COMP_WORDS[COMP_CWORD]}"
        local prev="${COMP_WORDS[COMP_CWORD-1]}"

        if [ "$prev" = "cd" ] || { [ "$COMP_CWORD" -ge 2 ] && [ "${COMP_WORDS[1]}" = "cd" ]; }; then
            local root="$(command rws config get workspace_root)"
            if [ -d "$root" ]; then
                local dirs
                dirs=$(cd "$root" && compgen -d -- "$cur" 2>/dev/null)
                COMPREPLY=($dirs)
            fi
            return
        fi

        if [ "$COMP_CWORD" -eq 1 ]; then
            COMPREPLY=($(compgen -W "cd" -- "$cur"))
        fi
    }
    complete -F _rws_completions rws
fi

# Zsh completion (eval'd to avoid bash parse errors on zsh glob syntax)
if [ -n "$ZSH_VERSION" ]; then
    eval '
    _rws() {
        local root="$(command rws config get workspace_root)"
        if [ "${words[2]}" = "cd" ]; then
            if [ -d "$root" ]; then
                local dirs=("$root"/*(/N:t))
                if [ ${#dirs[@]} -gt 0 ]; then
                    compadd -a dirs
                fi
            fi
        elif [ "$CURRENT" -eq 2 ]; then
            compadd cd
        fi
    }
    compdef _rws rws
    '
fi
