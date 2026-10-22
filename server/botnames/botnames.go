// Package botnames reads the optional names file that random bots can be
// named from (bot_names_file in config.yaml), so the owner can swap the
// built-in silly names for, say, the most popular biblical names.
package botnames

import (
	csv "encoding/csv"
	errors "errors"
	io "io"
	os "os"
	sort "sort"
	strconv "strconv"
	strings "strings"
)

// DefaultTop is how many of the file's names are used when the owner has
// not picked a number.
const DefaultTop = 200

var ErrNoColumns = errors.New( "names file needs a header with name and total_count columns" )

// Load reads a CSV with a header row naming at least a "name" and a
// "total_count" column -- the shape of babynames' all-names.csv and slices of
// it. Other columns (sex, say) are ignored. A name listed more than once,
// once per sex for instance, counts once with its counts added up. Names come
// back most popular first, ties in alphabetical order.
func Load( path string ) ( names []string , err error ) {
	file , err := os.Open( path )
	if err != nil { return }
	defer file.Close()
	reader := csv.NewReader( file )
	reader.FieldsPerRecord = -1
	header , err := reader.Read()
	if err != nil { return }
	name_column , count_column := -1 , -1
	for index , column := range header {
		switch strings.ToLower( strings.TrimSpace( column ) ) {
		case "name": name_column = index
		case "total_count": count_column = index
		}
	}
	if name_column < 0 || count_column < 0 {
		err = ErrNoColumns
		return
	}
	totals := map[ string ]int{}
	for {
		record , read_err := reader.Read()
		if errors.Is( read_err , io.EOF ) { break }
		if read_err != nil {
			err = read_err
			return
		}
		if len( record ) <= name_column || len( record ) <= count_column { continue }
		name := strings.TrimSpace( record[ name_column ] )
		count , parse_err := strconv.Atoi( strings.TrimSpace( record[ count_column ] ) )
		if name == "" || parse_err != nil { continue }
		if _ , seen := totals[ name ]; seen == false { names = append( names , name ) }
		totals[ name ] += count
	}
	sort.SliceStable( names , func( i int , j int ) bool {
		if totals[ names[ i ] ] != totals[ names[ j ] ] { return totals[ names[ i ] ] > totals[ names[ j ] ] }
		return names[ i ] < names[ j ]
	} )
	return
}

// Top is the first n of names, or DefaultTop of them when n is 0, or all of
// them when there are fewer.
func Top( names []string , n int ) ( result []string ) {
	if n <= 0 { n = DefaultTop }
	if n > len( names ) { n = len( names ) }
	result = names[ :n ]
	return
}
