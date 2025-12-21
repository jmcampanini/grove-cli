grs() {
    local output
    output=$(grove list --fzf | fzf --delimiter '\t' --with-nth 2 | cut -f1)
    if [ -n "$output" ]; then
        if command -v z &> /dev/null; then
            z "$output"
        else
            cd "$output"
        fi
    fi
}
