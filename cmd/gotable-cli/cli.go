package main

import (
	"fmt"
	"github.com/stevejiang/gotable/api/go/table"
	"strconv"
)

type client struct {
	c    *table.Client
	dbId uint8
}

func newClient() *client {
	var c = new(client)
	var err error
	c.c, err = table.Dial("tcp", *host)
	if err != nil {
		fmt.Println("dial failed: ", err)
		return nil
	}

	return c
}

func (c *client) do(cmd string, args []string) {
	fmt.Println(args)
}

func (c *client) use(args []string) error {
	//use <databaseId>
	if len(args) != 1 {
		return fmt.Errorf("Invalid number of arguments (%d)", len(args))
	}

	dbId, err := getDatabaseId(args[0])
	if err != nil {
		return err
	}

	fmt.Printf("OK, current database is %d\n", dbId)
	c.dbId = dbId

	return nil
}

func (c *client) get(args []string) error {
	//get <tableId> <rowKey> <colKey>
	if len(args) != 3 {
		return fmt.Errorf("Invalid number of arguments (%d)", len(args))
	}

	tableId, err := getTableId(args[0])
	if err != nil {
		return err
	}

	rowKey, err := extractKey(args[1])
	if err != nil {
		return err
	}
	colKey, err := extractKey(args[2])
	if err != nil {
		return err
	}

	r, err := c.c.Get(false, &table.OneArgs{tableId, []byte(rowKey), []byte(colKey), nil, 0, 0})
	if err != nil {
		return err
	}

	switch r.ErrCode {
	case table.EcodeOk:
		fmt.Printf("%q\n", r.Value)
	case table.EcodeNotExist:
		fmt.Println("(nil)")
	default:
		fmt.Printf("<Unknown error code %d>\n", r.ErrCode)
	}

	return nil
}

func (c *client) zget(args []string) error {
	//zget <tableId> <rowKey> <colKey>
	if len(args) != 3 {
		return fmt.Errorf("Invalid number of arguments (%d)", len(args))
	}

	tableId, err := getTableId(args[0])
	if err != nil {
		return err
	}

	rowKey, err := extractKey(args[1])
	if err != nil {
		return err
	}
	colKey, err := extractKey(args[2])
	if err != nil {
		return err
	}

	r, err := c.c.Get(true, &table.OneArgs{tableId, []byte(rowKey), []byte(colKey), nil, 0, 0})
	if err != nil {
		return err
	}

	switch r.ErrCode {
	case table.EcodeOk:
		fmt.Printf("%d\t%q\n", r.Score, r.Value)
	case table.EcodeNotExist:
		fmt.Println("(nil)")
	default:
		fmt.Printf("<Unknown error code %d>\n", r.ErrCode)
	}

	return nil
}

func (c *client) set(args []string) error {
	//set <tableId> <rowKey> <colKey> <value>
	if len(args) != 4 {
		return fmt.Errorf("Invalid number of arguments (%d)", len(args))
	}

	tableId, err := getTableId(args[0])
	if err != nil {
		return err
	}

	rowKey, err := extractKey(args[1])
	if err != nil {
		return err
	}
	colKey, err := extractKey(args[2])
	if err != nil {
		return err
	}
	value, err := extractKey(args[3])
	if err != nil {
		return err
	}

	r, err := c.c.Set(false, &table.OneArgs{tableId, []byte(rowKey), []byte(colKey),
		[]byte(value), 0, 0})
	if err != nil {
		return err
	}

	switch r.ErrCode {
	case table.EcodeCasNotMatch:
		fmt.Printf("CAS not match, the new cas is %d\n", r.Cas)
	default:
		fmt.Println("OK")
	}

	return nil
}

func (c *client) zset(args []string) error {
	//zset <tableId> <rowKey> <colKey> <value> <score>
	if len(args) != 5 {
		return fmt.Errorf("Invalid number of arguments (%d)", len(args))
	}

	tableId, err := getTableId(args[0])
	if err != nil {
		return err
	}

	rowKey, err := extractKey(args[1])
	if err != nil {
		return err
	}
	colKey, err := extractKey(args[2])
	if err != nil {
		return err
	}
	value, err := extractKey(args[3])
	if err != nil {
		return err
	}
	score, err := strconv.ParseInt(args[4], 10, 64)
	if err != nil {
		return err
	}

	r, err := c.c.Set(true, &table.OneArgs{tableId, []byte(rowKey), []byte(colKey),
		[]byte(value), score, 0})
	if err != nil {
		return err
	}

	switch r.ErrCode {
	case table.EcodeCasNotMatch:
		fmt.Printf("CAS not match, the new cas is %d\n", r.Cas)
	default:
		fmt.Println("OK")
	}

	return nil
}

func (c *client) scan(args []string) error {
	//scan <tableId> <rowKey> <colKey> <num>
	if len(args) != 4 {
		return fmt.Errorf("Invalid number of arguments (%d)", len(args))
	}

	tableId, err := getTableId(args[0])
	if err != nil {
		return err
	}

	rowKey, err := extractKey(args[1])
	if err != nil {
		return err
	}
	colKey, err := extractKey(args[2])
	if err != nil {
		return err
	}

	num, err := strconv.ParseInt(args[3], 10, 16)
	if err != nil {
		return err
	}

	var sa table.ScanArgs
	sa.Num = uint16(num)
	sa.Direction = 0
	sa.TableId = tableId
	sa.RowKey = []byte(rowKey)
	sa.ColKey = []byte(colKey)

	r, err := c.c.Scan(false, &sa)
	if err != nil {
		return err
	}

	if len(r.Reply) == 0 {
		fmt.Println("no record!")
	} else {
		for i := 0; i < len(r.Reply); i++ {
			var one = &r.Reply[i]
			fmt.Printf("%02d) [%q\t%q]\t[%d\t%q]\n", i,
				one.RowKey, one.ColKey, one.Score, one.Value)
		}
	}

	return nil
}

func (c *client) zscan(args []string) error {
	//zscan <tableId> <rowKey> <score> <colKey> <num>
	if len(args) != 5 {
		return fmt.Errorf("Invalid number of arguments (%d)", len(args))
	}

	tableId, err := getTableId(args[0])
	if err != nil {
		return err
	}

	rowKey, err := extractKey(args[1])
	if err != nil {
		return err
	}

	score, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return err
	}
	colKey, err := extractKey(args[3])
	if err != nil {
		return err
	}

	num, err := strconv.ParseInt(args[4], 10, 16)
	if err != nil {
		return err
	}

	var sa table.ScanArgs
	sa.Num = uint16(num)
	sa.Direction = 0
	sa.TableId = tableId
	sa.RowKey = []byte(rowKey)
	sa.ColKey = []byte(colKey)
	sa.Score = score

	r, err := c.c.Scan(true, &sa)
	if err != nil {
		return err
	}

	if len(r.Reply) == 0 {
		fmt.Println("no record!")
	} else {
		for i := 0; i < len(r.Reply); i++ {
			var one = &r.Reply[i]
			fmt.Printf("%02d) [%q\t%d\t%q]\t[%q]\n", i,
				one.RowKey, one.Score, one.ColKey, one.Value)
		}
	}

	return nil
}

func getTableId(arg string) (uint8, error) {
	tableId, err := strconv.Atoi(arg)
	if err != nil {
		return 0, fmt.Errorf("<tableId> %s is not a number", arg)
	}

	if tableId < 0 || tableId > 200 {
		return 0, fmt.Errorf("<tableId> %s is out of range (0 ~ 200)", arg)
	}

	return uint8(tableId), nil
}

func getDatabaseId(arg string) (uint8, error) {
	dbId, err := strconv.Atoi(arg)
	if err != nil {
		return 0, fmt.Errorf("<databaseId> %s is not a number", arg)
	}

	if dbId < 0 || dbId > 200 {
		return 0, fmt.Errorf("<databaseId> %s is out of range (0 ~ 200)", arg)
	}

	return uint8(dbId), nil
}

func extractKey(arg string) (string, error) {
	if arg[0] == '\'' || arg[0] == '"' {
		if len(arg) < 2 {
			return "", fmt.Errorf("Invalid key (%s)", arg)
		}

		if arg[0] != arg[len(arg)-1] {
			return "", fmt.Errorf("Invalid key (%s)", arg)
		}

		return arg[1 : len(arg)-1], nil
	}

	return arg, nil
}
