grc() {
    local output
    output=$(grove create "$*")
    if [ $? -eq 0 ]; then
        if command -v z &> /dev/null; then
            z "$output"
        else
            cd "$output"
        fi
    else
        echo "$output"
        return 1
    fi
}
